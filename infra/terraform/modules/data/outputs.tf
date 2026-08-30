output "rds_endpoint" {
  description = "RDS endpoint"
  value       = aws_db_instance.timescale.endpoint
}

output "rds_sg_id" {
  description = "RDS security group ID"
  value       = aws_security_group.rds.id
}
