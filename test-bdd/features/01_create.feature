Feature: CRUD Create
  In order to create instance-groups
  As an EKS cluster operator
  I need to submit the custom resource

  Scenario: Resources can be submitted
    Given an EKS cluster
    Then I create a resource instance-group.yaml
    And I create a resource instance-group-crd.yaml
    #And I create a resource instance-group-managed.yaml
    #And I create a resource instance-group-fargate.yaml

  Scenario: Create an instance-group with rollingUpdate strategy
    Given an EKS cluster
    When I create a resource instance-group.yaml
    Then the resource instance-group.yaml should be created
    And the resource instance-group.yaml should converge to selector .status.lifecycle=spot
    And the resource instance-group.yaml should converge to selector .status.currentState=ready
    And the resource instance-group.yaml condition NodesReady should be true
    And 2 nodes with selector test=bdd-test-rolling should be ready

  # Scenario: Create an instance-group with CRD strategy
  #   Given an EKS cluster
  #   When I create a resource instance-group-crd.yaml
  #   Then the resource instance-group-crd.yaml should be created
  #   And the resource instance-group-crd.yaml should converge to selector .status.lifecycle=spot
  #   And the resource instance-group-crd.yaml should converge to selector .status.currentState=ready
  #   And the resource instance-group-crd.yaml condition NodesReady should be true
  #   And 2 nodes with selector test=bdd-test-crd should be ready

  # Scenario: Create an instance-group with managed node group
  #   Given an EKS cluster
  #   When I create a resource instance-group-managed.yaml
  #   Then the resource instance-group-managed.yaml should be created
  #   And the resource instance-group-managed.yaml should converge to selector .status.currentState=ready
  #   And 2 nodes with selector test=bdd-test-managed should be ready

  # Scenario: Create a fargate profile with default execution role
  #   Given an EKS cluster
  #   Then I create a resource instance-group-fargate.yaml
  #   And the resource instance-group-fargate.yaml should be created
  #   And the fargate profile of the resource instance-group-fargate.yaml should be found

