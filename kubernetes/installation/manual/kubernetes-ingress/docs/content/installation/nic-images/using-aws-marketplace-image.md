---
title: Using the AWS Marketplace NGINX Ingress Controller Image
description: "This guide walks you through the steps to set up NGINX Ingress Controller using the AWS Marketplace."
weight: 200
doctypes: [""]
toc: true
docs: "DOCS-607"
---

## Before you being

Follow this guide to set up NGINX Ingress Controller using AWS Marketplace. This involves some extra steps to make sure everything works as it should.

{{< important >}}This guide focuses on EKS version 1.19. For EKS versions below 1.19, you'll need to adjust security settings in the NGINX Pod to ensure compatibility with marketplace images. Make sure you're using updated versions of `eksctl` and the AWS CLI.{{< /important >}}

{{< note >}}AWS Region US-West-1 doesn't support NGINX Ingress Controller.{{</note>}}

## Instructions

1. First, make sure your AWS EKS cluster is operational. If not, set one up using the AWS console or the `eksctl` tool. For step-by-step instructions, follow [this guide](https://docs.aws.amazon.com/eks/latest/userguide/getting-started-eksctl.html).

2. Create a new IAM role that will link to the service zccount for NGINX Ingress Controller. This role should have a policy that lets you monitor AWS NGINX Ingress Controller usage. Skipping this step will cause AWS NGINX Ingress Controller not to work. For more information, consult [AWS EKS IAM documentation](https://docs.aws.amazon.com/eks/latest/userguide/create-service-account-iam-policy-and-role.html) and [AWS Marketplace policy details](https://docs.aws.amazon.com/marketplace/latest/userguide/iam-user-policy-for-aws-marketplace-actions.html).

3. Link this IAM role to your EKS cluster service account. Doing this will annotate your service account Kubernetes object with the IAM role link.

{{< important >}}Associating your AWS EKS cluster with an OIDC provider is a prerequisite for creating your IAM service account.{{< /important >}}

## Step-by-step instructions using eksctl

Make sure you have an operational EKS cluster and that the namespace for your NGINX Ingress Controller is set up. If you don't have an EKS cluster yet, you'll need to create one.

1. Associate your EKS cluster with an OIDC IAM provider. Use your specific `--cluster <name`> and `--region <region>` values.

    ``` shell
    eksctl utils associate-iam-oidc-provider --region=eu-west-1 --cluster=json-eu-east1 --approve
    ```

2. Create an IAM role and a service account for your cluster. Replace `--name <name>`, `--namespace <name>`, and `--region <region>` with your values.

    ``` shell
    eksctl create iamserviceaccount --name nginx-ingress --namespace nginx-ingress --cluster json-test01 --region us-east-2 --attach-policy-arn arn:aws:iam::aws:policy/AWSMarketplaceMeteringRegisterUsage --approve
    ```

    This step creates the IAM role with the required policy, creates the service account if it doesn't exist, and adds the annotations needed for your AWS cluster. For additional details, consult the [AWS documentation](https://docs.aws.amazon.com/eks/latest/userguide/create-service-account-iam-policy-and-role.html). You don't need to apply any service account YAML files because `eksctl` handles that for you.

    ``` yaml
    apiVersion: v1
    kind: ServiceAccount
    metadata:
      annotations:
        EKS.amazonaws.com/role-arn: arn:aws:iam::001234567890:role/eksctl-json-us-west2-addon-iamserviceaccount-Role1-IJJ6CF9Y8IPY
      labels:
        app.kubernetes.io/managed-by: eksctl
      name: nginx-ingress
      namespace: nginx-ingress
    secrets:
    - name: nginx-ingress-token-zm728
    ```

    <br>

    Ensure the service account name matches the one in your _rbac.yaml_ file for manifest deployment.

    Here's what a sample _rbac.yaml_ file might look like:

    ``` yaml
    kind: ClusterRoleBinding
      apiVersion: rbac.authorization.k8s.io/v1
      metadata:
        name: nginx-ingress
      subjects:
      - kind: ServiceAccount
        name: nginx-ingress
        namespace: nginx-ingress
      roleRef:
        kind: ClusterRole
        name: nginx-ingress
        apiGroup: rbac.authorization.k8s.io
    ```

3. Sign in to the AWS ECR registry that specified in the instructions on the [AWS Marketplace portal](https://aws.amazon.com/marketplace/pp/prodview-fx3faxl7zqeau?sr=0-1&ref_=beagle&applicationId=AWSMPContessa).

    {{< img title="ECR pull instructions for NGINX Ingress Controller" src="./img/ecr-pull-instructions.png" >}}

    <br>

    {{< tip >}}For help with credentials, AWS Labs offers a credential helper. Check out [their GitHub repository](https://github.com/awslabs/amazon-ecr-credential-helper) for setup instructions.{{< /tip >}}

4. Update the image in the _nginx-plus-ingress.yaml_ manifest.
