# Terraform — Fleet Monitoring

Stack validado con `terraform 1.15.8` (ver `ci.yml` `hashicorp/setup-terraform 1.15.8`).

## Layout

```
infra/terraform/
  versions.tf, variables.tf, outputs.tf, main.tf
  modules/network  — VPC, 2 subnets (2 AZ), IGW, route table, SGs, db_subnet_group
  modules/data     — RDS Postgres 16 (db.t3.micro, 20GB, skip_final_snapshot) + SG 5432 solo ECS
  modules/services — ECS Fargate cluster + ecsTaskRole/ecsExecRole + task defs placeholder + ALB 80->8080
  terraform.tfvars.example
```

## Uso

```bash
terraform fmt -recursive infra/terraform
terraform fmt -check -recursive infra/terraform

cd infra/terraform
terraform init -backend=false
terraform validate

# con vars (sin commitear secretos)
cp terraform.tfvars.example terraform.tfvars   # editar sin password
TF_VAR_db_password='...' terraform plan
TF_VAR_db_password='...' terraform apply
```

Imágenes reales via `TF_VAR_image_api`, `TF_VAR_image_ingest`, `TF_VAR_image_consumer`
o `-var='image_api=ACCOUNT.dkr.ecr.us-east-1.amazonaws.com/fleet-api:TAG'`.
`variables.tf` no tiene defaults sensibles (`db_password` `sensitive=true` sin default).

## Tradeoffs

- **RDS Postgres 16 managed** (MVP): sin consumo RAM local, backups/parameter groups/encryption managed,
  costo `~$15/mes` `db.t3.micro` free-tier, `timescaledb` vía `rds.extensions`. Ideal para `16GB` dev.
- **ECS TimescaleDB self-host** alternativa: ahorra costo RDS, control total de extensión/PostGIS,
  pero requiere `~2GB` RAM + `gp3/EFS`, manejo de vacuuming/compresión/retention y failover manual.
  El comentario en `modules/data/main.tf` documenta la decisión; cambiar es solo reemplazar `aws_db_instance`.

`tfstate` local en MVP (`init -backend=false`). S3+DynamoDB `backend` queda post-MVP.
SGs: `5432` **solo** desde `ECS SG`, nunca `0.0.0.0/0` (ver `modules/data`).
