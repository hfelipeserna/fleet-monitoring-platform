output "alb_dns" {
  description = "DNS name of the ALB"
  value       = module.root.alb_dns
}

output "rds_endpoint" {
  description = "RDS endpoint"
  value       = module.root.rds_endpoint
}

output "vpc_id" {
  description = "VPC ID"
  value       = module.root.vpc_id
}

output "ecs_cluster_name" {
  description = "ECS cluster name"
  value       = module.root.ecs_cluster_name
}
