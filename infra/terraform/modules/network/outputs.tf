output "vpc_id" {
  description = "VPC ID"
  value       = aws_vpc.main.id
}

output "public_subnet_ids" {
  description = "Public subnet IDs (2 AZs)"
  value       = aws_subnet.public[*].id
}

output "ecs_sg_id" {
  description = "ECS security group ID (allowed to reach RDS)"
  value       = aws_security_group.ecs.id
}

output "alb_sg_id" {
  description = "ALB security group ID"
  value       = aws_security_group.alb.id
}

output "db_subnet_group_name" {
  description = "DB subnet group name"
  value       = aws_db_subnet_group.main.name
}
