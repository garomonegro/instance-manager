package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cucumber/godog"
	kdog "github.com/keikoproj/instance-manager/kubedog"
	util "github.com/keikoproj/instance-manager/kubedog/utilities"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var t kdog.Test

func TestMain(m *testing.M) {
	opts := godog.Options{
		Format:    "progress",
		Paths:     []string{"features"},
		Randomize: time.Now().UTC().UnixNano(), // randomize scenario execution order
	}

	// godog v0.10.0 (latest)
	status := godog.TestSuite{
		Name:                 "godogs",
		TestSuiteInitializer: InitializeTestSuite,
		ScenarioInitializer:  InitializeScenario,
		Options:              &opts,
	}.Run()

	if st := m.Run(); st > status {
		status = st
	}
	os.Exit(status)
}

func InitializeTestSuite(ctx *godog.TestSuiteContext) {

	ctx.BeforeSuite(func() {
		log.Info("BDD >> trying to delete any existing test instance-groups")
		t.DeleteAllResources()
	})

	ctx.AfterSuite(func() {
		log.Info("BDD >> trying to delete any existing test instance-groups")
		t.DeleteAllResources()
	})

	t.SetTestSuite(ctx)
}

func InitializeScenario(ctx *godog.ScenarioContext) {

	ctx.AfterStep(func(s *godog.Step, err error) {
		time.Sleep(time.Second * 5)
	})
	ctx.Step(`^the fargate profile of the resource ([^"]*) should be (found|not found)$`, theFargateProfileShouldBeFound)

	t.SetScenario(ctx)
	t.Run()
}

func theFargateProfileShouldBeFound(resourceFilename, state string) error {

	resourcePath := filepath.Join("templates", resourceFilename)
	args := util.NewTemplateArguments()

	gvr, resource, err := util.GetResourceFromYaml(resourcePath, t.KubeContext.RESTConfig, args)
	if err != nil {
		return err
	}

	const profileName = "test-bdd-profile-name"
	var (
		counter int
		exists  bool
	)
	for {
		exists = true
		if counter >= DefaultWaiterRetries {
			return errors.New("waiter timed out waiting for fargate profile state")
		}
		log.Infof("BDD >> waiting for resource %v/%v to become %v", resource.GetNamespace(), resource.GetName(), state)
		_, err := t.KubeContext.DynamicInterface.Resource(gvr.Resource).Namespace(resource.GetNamespace()).Get(resource.GetName(), metav1.GetOptions{})

		if err != nil {
			if !kerrors.IsNotFound(err) {
				return err
			}
			log.Infof("BDD >> %v/%v is not found: %v", resource.GetNamespace(), resource.GetName(), err)
			exists = false
		}
		switch state {
		case FargateProfileFound:
			if exists {
				log.Infof("BDD >> success - resource %v/%v found", resource.GetNamespace(), resource.GetName())
				return nil
			}
		case FargateProfileNotFound:
			if !exists {
				log.Infof("BDD >> success - resource %v/%v not found", resource.GetNamespace(), resource.GetName())
				return nil
			}
		}
		counter++
		time.Sleep(DefaultWaiterInterval)
	}
}
