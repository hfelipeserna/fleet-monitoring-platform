---
name: iac-and-cicd
description: Usa esta skill al trabajar en infraestructura y pipelines: Docker Compose local, Terraform/AWS, GitHub Actions y Fastlane/EAS para la app móvil. Trigger: docker, compose, dockerfile, terraform, aws, ecs, fargate, github actions, ci, cd, fastlane, eas, despliegue, deployment.
---

# Infraestructura, Cloud y CI/CD

## Reglas de oro del entorno (máquina 16 GB RAM)

- Preferí **robustez de diseño** sobre réplicas: un NATS JetStream mononodo con JetStream + file storage, una TimescaleDB, y un solo backend Go es suficiente para el MVP. No levantes clusters de 3 nodos "por bonito".
- Imágenes livianas: Go → multi-stage (`golang:1.22-alpine` build → `alpine:3.x` runtime). Web → build estático servido por nginx slim o el backend.
- Todo servicio con healthcheck y `depends_on: condition: service_healthy` donde aplique.
- **`docker compose config` SIEMPRE debe ser válido** al tocar infra.

## Docker Compose (local)

Servicios base: `nats` (con `-js` y persistencia en volumen), `timescaledb` (con init scripts en `/docker-entrypoint-initdb.d`), `ingestor` (backend Go), `agente` (Genkit, puede ser parte del backend), `web` (servidor estático). Env vars con `.env.example` versionado; `.env` real en `.gitignore`.

## Terraform (AWS, MVP)

- Módulos: `network` (VPC/subnets), `data` (NATS via ECS o serviço manejado; TimescaleDB via RDS Postgres 16 + extension timescaledb o una instancia contenedor — elegí y documentá el tradeoff), `services` (ECS Fargate del backend Go + servicio web).
- Terragrunt NO es necesario para el MVP. Mantené `terraform fmt` y `terraform validate` en verde. Outputs con URL de acceso. Nada de claves en los tfvars versionados.

## GitHub Actions (CI/CD)

- `.github/workflows/ci.yml`: `go vet`, tests Go, `npm` build/lint (web + mobile), `docker build` (backend/web), y `docker compose config -q` como validación de compose.
- Job opcional de **carga (k6)** que corre contra el entorno de CI o local levantado.
- Deploy: job `terraform apply` a un entorno protegido al mergear a `main` (branch protection). Secretos vía GitHub Secrets, nunca inline.

## Móvil CI/CD (Fastlane + EAS) — ver skill `expo-workflow` para detalle

- `Fastfile`: lanes `build_android` / `build_ios` (o `release`) + `upload` a EAS/AppStore. Lanes idempotentes con Fastlane match para signing si aplica. Si usas `eas build` cloud, Fastlane es wrapper `sh("eas build --platform android --profile preview --non-interactive --no-wait")`.
- Workflow móvil: `.github/workflows/mobile.yml` con `paths: ["mobile/**"]` + `typecheck` + `lint` + `npm test -- --run --coverage` + `expo export --platform all` y opcional `eas build` con `EXPO_TOKEN` secreto (ver `expo-workflow`). No disparar `mobile` si solo cambia `backend/`.
- Expo: preferí `eas build` cloud para no cargar 16GB RAM; Fastlane para signing/release local. `eas.json` `dev/preview/production` sin secretos; `EXPO_PUBLIC_API_URL` via env (`http://localhost:8080` vs `http://LAN_IP:8080` para Expo Go). Ver `expo-workflow` para `ipconfig getifaddr en0`, `--tunnel` vs LAN, `expo-doctor`.

## Verificación siempre

1. `docker compose config` exitoso.
2. `terraform fmt -check && terraform validate` (en dir terraform).
3. YAMLs de Actions parsean (no te inventes sintaxis de `uses:`; verificá que el action existe).
4. El README documenta: cómo levantar local, cómo correr k6, cómo desplegar cloud, cómo lanzar el build móvil.