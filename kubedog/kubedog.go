package kubedog

import (
	"fmt"
	"os"

	"github.com/cucumber/godog"
	kube "github.com/keikoproj/instance-manager/kubedog/kubernetes"
)

type Test struct {
	suiteContext    *godog.TestSuiteContext
	scenarioContext *godog.ScenarioContext
	KubeContext     kube.Client
}

const (
	testSucceededStatus int = 0
	testFailedStatus    int = 1
)

func (kdt *Test) Run() {

	// TODO: define default suite hooks if any, check that the suite context was set

	if kdt.scenarioContext == nil {
		fmt.Println("FATAL: kubedog.Test.scenarioContext was not set, use kubedog.Test.InitScenario")
		os.Exit(testFailedStatus)
	}

	// TODO: define default scenario hooks if any
	// TODO: define default step hooks if any

	//TODO: implement and define more steps
	kdt.scenarioContext.Step(`^an EKS cluster`, kdt.KubeContext.AnEKSCluster)
	kdt.scenarioContext.Step(`^I (create|delete) a resource ([^"]*)$`, kdt.KubeContext.ResourceOperation)
	kdt.scenarioContext.Step(`^the resource ([^"]*) should be (created|deleted)$`, kdt.KubeContext.ResourceShouldBe)
	kdt.scenarioContext.Step(`^the resource ([^"]*) should converge to selector ([^"]*)$`, kdt.KubeContext.ResourceShouldConvergeToSelector)
	kdt.scenarioContext.Step(`^the resource ([^"]*) condition ([^"]*) should be (true|false)$`, kdt.KubeContext.ResourceConditionShouldBe)
	kdt.scenarioContext.Step(`^I update a resource ([^"]*) with ([^"]*) set to ([^"]*)$`, kdt.KubeContext.UpdateResourceWithField)
	kdt.scenarioContext.Step(`^(\d+) nodes with selector ([^"]*) should be (found|ready)`, kdt.KubeContext.NodesWithSelectorShouldBe)
}

func (kdt *Test) SetTestSuite(testSuite *godog.TestSuiteContext) {

	kdt.suiteContext = testSuite
}

func (kdt *Test) SetScenario(scenario *godog.ScenarioContext) {

	kdt.scenarioContext = scenario
}

// Methods to be use by the user in the before or after suite, scenario or step hooks
// - TODO: implement more methods
func (kdt *Test) DeleteAllResources() {
	//Getting context
	kdt.KubeContext.AnEKSCluster()
	//Deleting resources defined in the templates folder
	kdt.KubeContext.DeleteAll()
}

//--
