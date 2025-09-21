# AWS Deployment (Terraform)

This Terraform deploys the proxy to AWS ECS Fargate in eu-west-1 (Dublin)
with an Internet-facing ALB. It creates: VPC, Subnets, ALB, ECS Cluster,
Task/Service, ECR repo, security groups, IAM, and CloudWatch logs.

## Prereqs
- Terraform >= 1.3, AWS CLI configured with access to eu-west-1.
- Docker for building and pushing the image.

## Quick Start
1) Initialize and plan
- `cd infra/terraform`
- `terraform init`
- `terraform apply -var "project_name=tailscale-proxy"` (accept defaults)

2) Build and push image
- Get ECR URL: `terraform output -raw ecr_repository_url`
- `docker build -t $(terraform output -raw ecr_repository_url):latest ../../`
- `aws ecr get-login-password --region eu-west-1 | docker login --username AWS --password-stdin $(terraform output -raw ecr_repository_url | cut -d'/' -f1)`
- `docker push $(terraform output -raw ecr_repository_url):latest`
- Force new deploy: `terraform apply -var "force_new_deployment=true"`

3) Get ALB URL
- `terraform output alb_dns_name`

## Configuring Env Vars / Secrets
- Plain env: pass `-var 'env_vars={AUTH_MODE="shared_secret",LISTEN_ADDR=":8080"}'`
- Secrets from SSM/Secrets Manager: create parameter/secret and pass ARNs via
  `-var 'secret_arns={AUTH_SHARED_SECRET="arn:aws:ssm:eu-west-1:...:parameter/prod/auth_secret"}'`.

## TLS (Optional)
Provide an ACM cert ARN in eu-west-1 and domain to enable HTTPS listener:
- `-var 'certificate_arn=arn:aws:acm:eu-west-1:...:certificate/...' -var 'domain_name=proxy.example.com'`

