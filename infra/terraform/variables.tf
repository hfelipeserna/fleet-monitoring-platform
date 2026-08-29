variable "project" {
  description = "Project name prefix for resource naming"
  type        = string
  default     = "fleet"
}

variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Deployment environment (dev, prod)"
  type        = string
  default     = "dev"
}

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "db_name" {
  description = "Initial database name"
  type        = string
  default     = "fleet"
}

variable "db_username" {
  description = "RDS master username"
  type        = string
  default     = "fleet"
}

variable "db_password" {
  description = "RDS master password (inject via TF_VAR_db_password or secrets manager, never commit)"
  type        = string
  sensitive   = true
}

variable "db_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.t3.micro"
}

variable "image_api" {
  description = "Container image for API service (override via TF_VAR_image_api)"
  type        = string
  default     = "public.ecr.aws/docker/library/nginx:alpine"
}

variable "image_ingest" {
  description = "Container image for ingest service (override via TF_VAR_image_ingest)"
  type        = string
  default     = "public.ecr.aws/docker/library/nginx:alpine"
}

variable "image_consumer" {
  description = "Container image for consumer service (override via TF_VAR_image_consumer)"
  type        = string
  default     = "public.ecr.aws/docker/library/nginx:alpine"
}
