output "vpc_id" {
  description = "ID of the VPC"
  value       = module.network.vpc_id
}

output "alb_dns" {
  description = "DNS name of the ALB"
  value       = module.services.alb_dns
}

output "rds_endpoint" {
  description = "RDS endpoint"
  value       = module.data.rds_endpoint
}

output "ecs_cluster_name" {
  description = "ECS cluster name"
  value       = module.services.ecs_cluster_name
}
