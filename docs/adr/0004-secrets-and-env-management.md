# ADR-0004 — Gestión de secretos y configuración por entorno

- **Fecha:** 2026-08-21
- **Estado:** Aceptado (consolida condiciones dispersas de ADR-0002 cond.6, ADR-0003 cond.1 y dictamen `security` 2026-08-21)
- **Decisores:** `architect` (dictamen de `security` previo sobre gestión de API keys incorporado)

## Contexto

La plataforma maneja secretos reales (`GEMINI_API_KEY`, credenciales de TimescaleDB, futuros tokens EAS/Fastlane y credenciales AWS) en tres entornos con requisitos distintos: desarrollo local, CI/CD (GitHub Actions) y despliegue cloud (Terraform/AWS). El repositorio es **público**: un secreto commiteado es cosechado por bots en minutos. AGENTS.md fija "configuración vía env vars, nunca hardcodeada" como convención, pero no define dónde viven los *valores* por entorno, quién rota qué, ni cómo se detecta un leak.

## Decisión

**Los valores secretos nunca viven en git; cada entorno tiene su mecanismo, con `.env.example` como único contrato declarativo:**

| Entorno | Almacén de valores | Cómo llega al proceso |
|---|---|---|
| Local | `.env` (git-ignored, creado desde `.env.example`) | env vars del proceso/compose |
| CI/CD | GitHub Actions repository secrets | `${{ secrets.* }}` inyectadas a steps |
| Cloud (prod) | AWS Secrets Manager / SSM Parameter Store, gestionado por Terraform | task definition / runtime injection |

Reglas transversales:

1. `.env.example` declara **nombres y contratos** de variables (nunca valores reales); es el único archivo de entorno versionado.
2. Detección obligatoria en dos capas: secret-guard del plugin local (edición) + **gitleaks en CI** (bloquea merge). Push Protection de GitHub activada en el repo.
3. Rotación: toda key se considera comprometida si aparece en cualquier log, trace o captura; procedimiento por tipo de secreto documentado en este ADR al agregarse cada uno.
4. Principio de mínimo alcance: cada servicio recibe solo las variables que necesita (ADR-0003: `GEMINI_API_KEY` solo el bin `agent`; fuera del contrato de IaC).

## Opciones evaluadas y descartadas

| Opción | Veredicto | Motivo |
|---|---|---|
| Secretos en `.env` commiteado / cifrado en repo | Prohibida | Repo público; git-crypt/age añade fricción sin eliminar el riesgo de exposición |
| HashiCorp Vault | Descartada para MVP | Operar un Vault excede 16 GB RAM/dev y el alcance del MVP; path de adopción anotado si hay multi-tenant real |
| Hardcodear en IaC (`.tfvars`) | Prohibida | `.tfvars` con valores reales queda fuera de git (`.gitignore`); solo `*.tfvars.example` |
| Env vars + Secrets Manager (elegida) | Adoptada | Estándar de la plataforma AWS ya decidida; costo marginal nulo en free tier; rotación/audit nativos |

## Condiciones obligatorias

1. `.gitignore` cubre `.env*` con única excepción `!.env.example` (sin patrones parciales que dejen huecos tipo `.env.production`).
2. gitleaks corre en CI en cada PR/push con historia completa (`fetch-depth: 0`) y es **condición de merge** junto con los status checks de main.
3. Compose referencia secretos solo por interpolación (`${GEMINI_API_KEY}`), jamás valores literales; `docker compose config` no debe revelar ningún secreto.
4. Los secretos de producción se crean **solo** por Terraform (Secrets Manager) o manualmente con doble revisión durante el MVP; nunca `terraform apply` con valores en línea.
5. Al agregar cualquier secreto nuevo a este repo: entrada en `.env.example` + actualización de la tabla de rotación en este ADR.
6. Migración a Vertex AI/service accounts (eliminando la key plana) es condición previa a uso productivo real (heredada de ADR-0003 cond.7).

## Consecuencias

**Positivas:**
- Un solo contrato declarativo (`.env.example`) legible por humanos y verificable en PRs; onboarding = copiar y llenar.
- Detección de leaks automática y bloqueante antes del push/merge, no dependiente de disciplina humana.
- Path de migración claro: cambiar el almacén por entorno no cambia el contrato de variables ni el código (todo consume env vars).

**Negativas:**
- Dos capas de detección (plugin + CI) que mantener; falsos positivos ocasionales del patrón matcher (excepciones documentadas en el plugin).
- Secrets Manager introduce una dependencia AWS adicional en prod (costo ~$0.40/secreto/mes — asumido).
- La rotación manual durante el MVP depende de disciplina; sin automatizar hasta existir infra de producción.

## Referencias

- Dictamen `security` 2026-08-21 (hallazgo alta: GEMINI_API_KEY sin salvaguardas; OWASP A07).
- ADR-0002 cond.6 (`.env.example` como contrato único, umbral <50 vars).
- ADR-0003 cond.1 y cond.7 (gestión/rotación de la key del agente, path Vertex AI).
