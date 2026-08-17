---
description: Corre el script k6 de caos/carga (300 vehículos, 10% duplicados, 5% errores) y reporta el comportamiento de dedup y resiliencia.
agent: devops
---

Ejecuta la batería k6 de caos/carga del proyecto.

1. Verifica que el stack esté levantado (`docker compose ps`); si no, levanta con `docker compose up -d` (tras `verify` que `docker compose config` pase).
2. Corre `k6 run` sobre el/los script(s) de `tests/load/` (constant-vus, 300 VU; 10% client_event_id duplicados, 5% payloads inválidos). Adjunta thresholds como umbrales.
3. Verifica dedup: compara la cuenta de eventos aceptados por el API vs filas únicas en telemetría (consulta a la DB o endpoint de health/statistics). Debe haber exactamente 1 fila por event_id a pesar de los duplicados.
4. Reporta: requests/sec, p95 latencia, checks, y el resultado de la verificación de dedup (cuántos duplicados rechazados/absorbidos). Si algún threshold se rompe, dímelo con causa probable y propuesta de fix.