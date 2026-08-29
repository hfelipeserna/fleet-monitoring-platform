variable "project" {
  description = "Project name"
  type        = string
}

variable "environment" {
  description = "Environment"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID"
  type        = string
}

variable "subnet_ids" {
  description = "Subnet IDs for DB subnet group fallback"
  type        = list(string)
  default     = []
}

variable "ecs_sg_id" {
  description = "ECS security group ID allowed to access RDS"
  type        = string
}

variable "db_subnet_group_name" {
  description = "DB subnet group name from network module"
  type        = string
}

variable "db_name" {
  description = "Initial DB name"
  type        = string
  default     = "fleet"
}

variable "db_username" {
  description = "RDS master username"
  type        = string
}

variable "db_password" {
  description = "RDS master password"
  type        = string
  sensitive   = true
}

variable "db_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.t3.micro"
}
