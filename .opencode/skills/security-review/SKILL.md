---
name: security-review
description: Usa esta skill al auditar seguridad en cualquier capa del repo: secretos, inyección, authN/authZ, exposición de GPS/PII, dependencias CVE, TLS, hardening IaC y data-at-rest móvil. Trigger: seguridad, security, auditoría, secret, CVE, inyección, XSS, sql injection, prompt injection, CORS, cifrado, TLS.
---

# Security review (baseline de proyecto)

Baseline mínima para que algo sea "no inseguro". Se aplica por componente: backend, eventos, IA, web, móvil, infra.

## Backend Go

- **Secretos**: cero valores no en env vars. Un token leído de env jamás en logs, headers de debug ni estructura serializada. `.env.example` con placeholders.
- **Inyección**: SQL solo con `$1` parametrizado (pgx). Paths: construir con `filepath.Join` y validar contra root (`path.Clean` + prefix check). No confiar en input del dispositivo para armar subjects de NATS sin validar formato.
- **Errores**: nunca exponer stack/query a la API. Traducir a códigos seguros. Loggear server-side.
- **DoS**: `http.Server` con timeouts (Read/Write/Idle), límite de body, rate limit razonable en ingesta y chat IA.

## NATS / eventos

- Credenciales de conexión: via env, con TLS para entornos que no sean local-dev. Auth para los streams no triviales (dejar claro en compose cuando sea dev-only).
- `Nats-Msg-Id` y `event_id` random (UUID v4) para evitar duplicados/colisión.
- Subjects de telemetría: validar `device_id` antes de publicar/consumir (evitar enum por sujeto inyectado).

## Agente IA

- **Prompt injection**: nunca concatenar el input del usuario al system prompt sin contenerlo como *data*; instruir que ignore instrucciones de negación/re-write. Tools solo de LECTURA del estado de la flota.
- Límites: `maxOutputTokens` acotado, timeout, y no exponer prompt internos en el response.

## Web

- CORS cerrado a orígenes conocidos (no `*` con credentials).
- No guardar tokens en `localStorage` ni en el state global si no hace falta (mejor cookie httpOnly vía backend, o al menos documentar el tradeoff).
- Render seguro: React escapa por defecto; no usar `dangerouslySetInnerHTML` sin sanitizar.

## Móvil

- GPS/posición en el repo local: cifrado a nivel de app (Expo SecureStore para el sync token; DB local con los datos del trayecto del conductor — evaluar cifrado en repositorio o al menos pedido de consentimiento del conductor).
- `client_event_id` = UUID v4 (no secuencial/predecible).
- Sync requiere auth; token nunca hardcodeado ni en logs de Expo.

## IaC / CI

- Security groups: nada de `0.0.0.0/0` sin justificación (público solo web/API, DB en subnet privada).
- Secrets: GitHub Secrets + `secrets: inherit` consciente; jamás en `.tfvars` versionados ni `echo` en steps. Disks con encrypt y backups habilitados.
- IAM: least privilege (policy con actions mínimas, resource params).

## Checklist del auditor

1. ¿Ningún secreto en repo/versión/línea de command?
2. ¿SQL/LLM/paths inyección cómplices en las capas de datos e IA?
3. ¿authN/authZ presente en lo que expone datos de la flota (web, móvil, API)?
4. ¿Errores no filtran internos?
5. ¿Hardening razonable en IaC/CI (SGs, secrets, cifrado)?