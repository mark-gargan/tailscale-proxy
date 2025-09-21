variable "region" {
  description = "AWS region"
  type        = string
  default     = "eu-west-1"
}

variable "project_name" {
  description = "Project/name prefix for resources"
  type        = string
}

variable "vpc_cidr" {
  description = "VPC CIDR"
  type        = string
  default     = "10.20.0.0/16"
}

variable "container_port" {
  description = "Container listening port"
  type        = number
  default     = 8080
}

variable "desired_count" {
  description = "Number of ECS tasks"
  type        = number
  default     = 1
}

variable "health_check_path" {
  description = "ALB health check path"
  type        = string
  default     = "/"
}

variable "cpu" {
  description = "Task CPU units"
  type        = number
  default     = 256
}

variable "memory" {
  description = "Task memory (MiB)"
  type        = number
  default     = 512
}

variable "env_vars" {
  description = "Plain environment variables for the container"
  type        = map(string)
  default     = {}
}

variable "secret_arns" {
  description = "Map of NAME -> SSM/Secrets Manager ARN for container secrets"
  type        = map(string)
  default     = {}
}

variable "certificate_arn" {
  description = "ACM certificate ARN for HTTPS listener (optional)"
  type        = string
  default     = ""
}

variable "domain_name" {
  description = "Domain name for ALB (optional, used for ref only)"
  type        = string
  default     = ""
}

variable "force_new_deployment" {
  description = "Force ECS new deployment to pull latest image"
  type        = bool
  default     = false
}

