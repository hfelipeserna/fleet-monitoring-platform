---
description: Levanta/apaga el stack local con Docker Compose y muestra el estado de salud de los servicios.
agent: devops
---

Gestión del entorno local con Docker Compose.

Acción: $ARGUMENTS (por defecto: up).

- `up`: valida `docker compose config` y levanta el stack (`up -d --build`), espera que NATS, TimescaleDB y el backend queden healthy (`docker compose ps`), y muestra las URLs de acceso (web, SSE, health).
- `down`: detiene el stack.
- `down -v`: requiere confirmación explícita (pide al usuario) porque borra volúmenes de datos.
- `logs <svc>`: muestra logs del servicio indicado.

Restricción: no levantes servicios de más; la máquina tiene 16 GB RAM. Si un servicio no es necesario para lo que busca el usuario, ni lo menciones como arranque por defecto.