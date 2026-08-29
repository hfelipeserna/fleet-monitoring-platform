variable "project" {
  description = "Project name prefix"
  type        = string
}

variable "environment" {
  description = "Environment"
  type        = string
}

variable "vpc_cidr" {
  description = "VPC CIDR block"
  type        = string
}
