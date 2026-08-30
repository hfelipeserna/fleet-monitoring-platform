variable "project" {
  description = "Project name"
  type        = string
}

variable "environment" {
  description = "Environment"
  type        = string
}

variable "region" {
  description = "AWS region for log group"
  type        = string
  default     = "us-east-1"
}

variable "vpc_id" {
  description = "VPC ID"
  type        = string
}

variable "subnet_ids" {
  description = "Subnet IDs for ECS tasks and ALB"
  type        = list(string)
}

variable "ecs_sg_id" {
  description = "ECS tasks security group ID"
  type        = string
}

variable "image_api" {
  description = "API image (inject real via TF_VAR_image_api)"
  type        = string
  default     = "public.ecr.aws/docker/library/nginx:alpine"
}

variable "image_ingest" {
  description = "Ingest image (inject real via TF_VAR_image_ingest)"
  type        = string
  default     = "public.ecr.aws/docker/library/nginx:alpine"
}

variable "image_consumer" {
  description = "Consumer image (inject real via TF_VAR_image_consumer)"
  type        = string
  default     = "public.ecr.aws/docker/library/nginx:alpine"
}

variable "desired_count" {
  description = "Desired ECS tasks"
  type        = number
  default     = 1
}
