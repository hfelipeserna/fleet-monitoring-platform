# MVP decision: RDS Postgres 16 managed vs ECS TimescaleDB self-host
# Tradeoff: RDS reduces local 16GB RAM pressure, gives automated backups/parameter groups
# and managed extensions (timescaledb via rds.extensions), at ~$15/mo for db.t3.micro
# + 20GB. Alternative self-host on ECS/Fargate would need 2GB+ RAM + EBS, EFS or
# instance storage, manual vacuuming/compression and replica handling, but saves
# managed cost and keeps PostGIS+Timescale fully controlled. MVP picks RDS for
# simplicity; post-MVP can flip to ECS Timescale container if cost or
# extension version pinning requires it. No 0.0.0.0/0 on 5432 — only ECS SG.

resource "aws_security_group" "rds" {
  name        = "${var.project}-${var.environment}-rds"
  description = "RDS SG - only ECS tasks can reach 5432"
  vpc_id      = var.vpc_id

  ingress {
    description     = "Postgres from ECS SG only"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [var.ecs_sg_id]
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.project}-${var.environment}-rds"
  }
}

resource "aws_db_instance" "timescale" {
  identifier              = "${var.project}-${var.environment}"
  engine                  = "postgres"
  engine_version          = "16.3"
  instance_class          = var.db_instance_class
  allocated_storage       = 20
  storage_type            = "gp3"
  storage_encrypted       = true
  db_name                 = var.db_name
  username                = var.db_username
  password                = var.db_password
  db_subnet_group_name    = var.db_subnet_group_name
  vpc_security_group_ids  = [aws_security_group.rds.id]
  publicly_accessible     = false
  skip_final_snapshot     = true
  backup_retention_period = 7
  deletion_protection     = false
  apply_immediately       = true

  tags = {
    Name = "${var.project}-${var.environment}-db"
  }
}
