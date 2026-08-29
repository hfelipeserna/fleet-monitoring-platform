provider "aws" {
  region = var.region
}

module "network" {
  source      = "./modules/network"
  project     = var.project
  environment = var.environment
  vpc_cidr    = var.vpc_cidr
}

module "data" {
  source               = "./modules/data"
  project              = var.project
  environment          = var.environment
  vpc_id               = module.network.vpc_id
  subnet_ids           = module.network.public_subnet_ids
  ecs_sg_id            = module.network.ecs_sg_id
  db_subnet_group_name = module.network.db_subnet_group_name
  db_name              = var.db_name
  db_username          = var.db_username
  db_password          = var.db_password
  db_instance_class    = var.db_instance_class
}

module "services" {
  source         = "./modules/services"
  project        = var.project
  environment    = var.environment
  region         = var.region
  vpc_id         = module.network.vpc_id
  subnet_ids     = module.network.public_subnet_ids
  ecs_sg_id      = module.network.ecs_sg_id
  image_api      = var.image_api
  image_ingest   = var.image_ingest
  image_consumer = var.image_consumer
}
