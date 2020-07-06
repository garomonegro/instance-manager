package kube

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	util "github.com/keikoproj/instance-manager/kubedog/utilities"
	"github.com/keikoproj/instance-manager/test-bdd/testutil"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	KubeInterface    kubernetes.Interface
	DynamicInterface dynamic.Interface
	RESTConfig       *rest.Config
	// TODO: use to map created resources with their name to avoid having to read the file again or be limited to the last resource created
	// Resources map[string]resourceContext
}

/*
type resourceContext struct {
	Resource *unstructured.Unstructured
	GVR      *meta.RESTMapping
}
*/

const (
	OperationCreate = "create"
	OperationUpdate = "update"
	OperationDelete = "delete"

	ResourceStateCreated = "created"
	ResourceStateDeleted = "deleted"

	NodeStateReady = "ready"
	NodeStateFound = "found"

	// TODO: have the user customize this?
	DefaultWaiterInterval = time.Second * 30
	DefaultWaiterRetries  = 40
	//--
)

func (kc *Client) AnEKSCluster() error {
	var (
		home, _        = os.UserHomeDir()
		kubeconfigPath = filepath.Join(home, ".kube", "config")
	)

	if exported := os.Getenv("KUBECONFIG"); exported != "" {
		kubeconfigPath = exported
	}

	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		return errors.Errorf("BDD >> expected kubeconfig to exist for create operation, '%v'", kubeconfigPath)
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Fatal("Unable to construct dynamic client", err)
	}

	_, err = client.Discovery().ServerVersion()
	if err != nil {
		return err
	}

	kc.KubeInterface = client
	kc.DynamicInterface = dynClient
	kc.RESTConfig = config

	return nil
}

func (kc *Client) ResourceOperation(operation, resourceFileName string) error {

	resourcePath := filepath.Join("templates", resourceFileName)
	args := util.NewTemplateArguments()

	gvr, resource, err := util.GetResourceFromYaml(resourcePath, kc.RESTConfig, args)
	if err != nil {
		return err
	}

	switch operation {
	case OperationCreate:
		_, err = kc.DynamicInterface.Resource(gvr.Resource).Namespace(resource.GetNamespace()).Create(resource, metav1.CreateOptions{})
		if err != nil {
			if kerrors.IsAlreadyExists(err) {
				// already created
				break
			}
			return err
		}
	case OperationDelete:
		err = kc.DynamicInterface.Resource(gvr.Resource).Namespace(resource.GetNamespace()).Delete(resource.GetName(), &metav1.DeleteOptions{})
		if err != nil {
			if kerrors.IsNotFound(err) {
				// already deleted
				break
			}
			return err
		}
	}
	return nil
}

func (kc *Client) ResourceShouldBe(resourceFileName string, state string) error {
	var (
		exists  bool
		counter int
	)
	resourcePath := filepath.Join("templates", resourceFileName)
	args := util.NewTemplateArguments()
	gvr, resource, err := util.GetResourceFromYaml(resourcePath, kc.RESTConfig, args)
	if err != nil {
		return err
	}

	for {
		exists = true
		if counter >= DefaultWaiterRetries {
			return errors.New("waiter timed out waiting for resource state")
		}
		log.Infof("BDD >> waiting for resource %v/%v to become %v", resource.GetNamespace(), resource.GetName(), state)

		_, err := kc.DynamicInterface.Resource(gvr.Resource).Namespace(resource.GetNamespace()).Get(resource.GetName(), metav1.GetOptions{})
		if err != nil {
			if !kerrors.IsNotFound(err) {
				return err
			}
			log.Infof("BDD >> %v/%v is not found: %v", resource.GetNamespace(), resource.GetName(), err)
			exists = false
		}

		switch state {
		case ResourceStateDeleted:
			if !exists {
				log.Infof("BDD >> %v/%v is deleted", resource.GetNamespace(), resource.GetName())
				return nil
			}
		case ResourceStateCreated:
			if exists {
				log.Infof("BDD >> %v/%v is created", resource.GetNamespace(), resource.GetName())
				return nil
			}
		}
		counter++
		time.Sleep(DefaultWaiterInterval)
	}
}

func (kc *Client) ResourceShouldConvergeToSelector(resourceFileName string, selector string) error {
	var (
		counter  int
		split    = util.DeleteEmpty(strings.Split(selector, "="))
		key      = split[0]
		keySlice = util.DeleteEmpty(strings.Split(key, "."))
		value    = split[1]
	)

	resourcePath := filepath.Join("templates", resourceFileName)
	args := util.NewTemplateArguments()
	gvr, resource, err := util.GetResourceFromYaml(resourcePath, kc.RESTConfig, args)
	if err != nil {
		return err
	}

	for {
		if counter >= DefaultWaiterRetries {
			return errors.New("waiter timed out waiting for resource")
		}

		log.Infof("BDD >> waiting for resource %v/%v to converge to %v=%v", resource.GetNamespace(), resource.GetName(), key, value)
		resource, err := kc.DynamicInterface.Resource(gvr.Resource).Namespace(resource.GetNamespace()).Get(resource.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		if val, ok, err := unstructured.NestedString(resource.UnstructuredContent(), keySlice...); ok {
			if err != nil {
				return err
			}
			if strings.EqualFold(val, value) {
				break
			}
		}
		counter++
		time.Sleep(DefaultWaiterInterval)
	}

	return nil
}

func (kc *Client) ResourceConditionShouldBe(resourceFileName string, cType string, cond string) error {
	var (
		counter int
	)

	resourcePath := filepath.Join("templates", resourceFileName)
	args := util.NewTemplateArguments()
	gvr, resource, err := util.GetResourceFromYaml(resourcePath, kc.RESTConfig, args)
	if err != nil {
		return err
	}

	for {
		if counter >= DefaultWaiterRetries {
			return errors.New("waiter timed out waiting for resource state")
		}
		log.Infof("BDD >> waiting for resource %v/%v to meet condition %v=%v", resource.GetGenerateName(), resource.GetName(), cType, cond)
		resource, err := kc.DynamicInterface.Resource(gvr.Resource).Namespace(resource.GetGenerateName()).Get(resource.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		if conditions, ok, err := unstructured.NestedSlice(resource.UnstructuredContent(), "status", "conditions"); ok {
			if err != nil {
				return err
			}

			for _, c := range conditions {
				condition, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				tp, found := condition["type"]
				if !found {
					continue
				}
				condType, ok := tp.(string)
				if !ok {
					continue
				}
				if condType == cType {
					status := condition["status"].(string)
					if corev1.ConditionStatus(status) == corev1.ConditionTrue {
						return nil
					}
				}
			}
		}
		counter++
		time.Sleep(DefaultWaiterInterval)
	}
}

func (kc *Client) NodesWithSelectorShouldBe(nodeCount int, selector, state string) error {
	var (
		counter int
		found   bool
	)

	for {
		var (
			conditionNodes int
			opts           = metav1.ListOptions{
				LabelSelector: selector,
			}
		)

		if counter >= DefaultWaiterRetries {
			return errors.New("waiter timed out waiting for nodes")
		}

		log.Infof("BDD >> waiting for %v nodes to be %v with selector %v", nodeCount, state, selector)
		nodes, err := kc.KubeInterface.CoreV1().Nodes().List(opts)
		if err != nil {
			return err
		}

		switch state {
		case NodeStateFound:
			if len(nodes.Items) == nodeCount {
				log.Infof("BDD >> found %v nodes", nodeCount)
				found = true
			}
		case NodeStateReady:
			for _, node := range nodes.Items {
				if testutil.IsNodeReady(node) {
					conditionNodes++
				}
			}
			if conditionNodes == nodeCount {
				log.Infof("BDD >> found %v ready nodes", nodeCount)
				found = true
			}
		}

		if found {
			break
		}

		counter++
		time.Sleep(DefaultWaiterInterval)
	}
	return nil
}

func (kc *Client) UpdateResourceWithField(resourceFilename, key string, value string) error {
	var (
		keySlice     = testutil.DeleteEmpty(strings.Split(key, "."))
		overrideType bool
		intValue     int64
	)

	resourcePath := filepath.Join("templates", resourceFilename)
	args := util.NewTemplateArguments()
	gvr, resource, err := util.GetResourceFromYaml(resourcePath, kc.RESTConfig, args)
	if err != nil {
		return err
	}

	n, err := strconv.ParseInt(value, 10, 64)
	if err == nil {
		overrideType = true
		intValue = n
	}

	updateTarget, err := kc.DynamicInterface.Resource(gvr.Resource).Namespace(resource.GetNamespace()).Get(resource.GetName(), metav1.GetOptions{})
	if err != nil {
		return err
	}

	if overrideType {
		unstructured.SetNestedField(updateTarget.UnstructuredContent(), intValue, keySlice...)
	} else {
		unstructured.SetNestedField(updateTarget.UnstructuredContent(), value, keySlice...)
	}

	_, err = kc.DynamicInterface.Resource(gvr.Resource).Namespace(resource.GetNamespace()).Update(updateTarget, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	time.Sleep(3 * time.Second)
	return nil
}

// TODO: improve efficiency, only delete if the resources exist
func (kc *Client) DeleteAll() error {
	var deleteFn = func(path string, info os.FileInfo, err error) error {

		if info.IsDir() || filepath.Ext(path) != ".yaml" {
			return nil
		}

		args := util.NewTemplateArguments()

		gvr, resource, err := util.GetResourceFromYaml(path, kc.RESTConfig, args)
		if err != nil {
			return err
		}

		kc.DynamicInterface.Resource(gvr.Resource).Namespace(resource.GetNamespace()).Delete(resource.GetName(), &metav1.DeleteOptions{})
		log.Infof("BDD >> submitted deletion for %v/%v", resource.GetNamespace(), resource.GetName())
		return nil
	}

	var waitFn = func(path string, info os.FileInfo, err error) error {
		var (
			counter int
		)

		if info.IsDir() || filepath.Ext(path) != ".yaml" {
			return nil
		}

		args := util.NewTemplateArguments()

		gvr, resource, err := util.GetResourceFromYaml(path, kc.RESTConfig, args)
		if err != nil {
			return err
		}

		for {
			if counter >= DefaultWaiterRetries {
				return errors.New("waiter timed out waiting for deletion")
			}
			log.Infof("BDD >> waiting for resource deletion of %v/%v", resource.GetNamespace(), resource.GetName())
			_, err := kc.DynamicInterface.Resource(gvr.Resource).Namespace(resource.GetNamespace()).Get(resource.GetName(), metav1.GetOptions{})
			if err != nil {
				if kerrors.IsNotFound(err) {
					log.Infof("BDD >> resource %v/%v is deleted", resource.GetNamespace(), resource.GetName())
					break
				}
			}
			counter++
			time.Sleep(DefaultWaiterInterval)
		}
		return nil
	}

	if err := filepath.Walk("templates", deleteFn); err != nil {
		return err
	}

	if err := filepath.Walk("templates", waitFn); err != nil {
		return err
	}

	return nil
}
