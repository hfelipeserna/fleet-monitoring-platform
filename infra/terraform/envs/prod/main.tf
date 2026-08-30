module "root" {
  source            = "../.."
  project           = var.project
  environment       = "prod"
  region            = var.region
  vpc_cidr          = var.vpc_cidr
  db_name           = var.db_name
  db_username       = var.db_username
  db_password       = var.db_password
  db_instance_class = var.db_instance_class
}
