# ADR-0005 — Backend como monolito modular (un módulo Go, 4 bins) en vez de microservicios; resiliencia con circuit breakers gobreaker

- **Fecha:** 2026-08-22
- **Estado:** Aceptado (con condiciones). Nota de gobernanza: el dictamen dedicado de `scalability` para este ADR no pudo ejecutarse en la sesión del 2026-08-22 (3 intentos del subagente sin respuesta útil); la base numérica se **hereda de los dictámenes previos de `scalability` ya incorporados en ADR-0001 y ADR-0002**. El refrendo dedicado queda como condición 8 y es bloqueante antes de cerrar el SPEC de ingesta.
- **Decisores:** `architect` (+ evidencia numérica de dictámenes previos de `scalability` incorporados en ADR-0001/0002)

## Contexto

El enunciado canónico pide literalmente: *"Resiliencia: Implementar Circuit Breakers en la comunicación entre microservicios"* (`docs/PRUEBA-TECNICA.md`, sec. 4.A). Las decisiones ya aceptadas fijan: NATS JetStream como backbone (ADR-0001), un monorepo con **un solo módulo Go** desplegado como **4 bins** (`cmd/ingest`, `cmd/consumer`, `cmd/api`, `cmd/agent`; ADR-0002) y Genkit+Gemini para el agente (ADR-0003, que ya exige gobreaker sobre la llamada al LLM).

Falta explicitar tres cosas: (1) por qué ese diseño **no son microservicios clásicos**, (2) qué se pierde al no adoptarlos y cuánto importa aquí, y (3) **dónde viven legítimamente los circuit breakers cuando no existe comunicación RPC entre servicios**. Esta decisión desvía el literal del enunciado ("microservicios") manteniendo íntegro su requisito de fondo (resiliencia ante fallos parciales), y esa desviación debe quedar justificada y trazable.

Datos numéricos relevantes ya dictaminados e incorporados:

- Carga de referencia (ADR-0001): 5.000 dispositivos @ 1 evento/5 s = **1.000 msg/s sostenidos**, picos 2-3× (2.000-3.000 msg/s); horizonte de diseño 10.000-50.000 dispositivos (2.000-10.000 msg/s); payload 200-500 B.
- Techo JetStream async ≈ 370k msg/s R1 file: la carga objetivo usa el **0,3-2 %** del cuello de botella del broker (ADR-0001).
- Single-writer TimescaleDB: **15-50k filas/s** con batching/CopyFrom (ADR-0001).
- RSS 50-150 MB/bin; footprint vivo dev esperado **6-9 GB de los 16** (ADR-0002, cond. 7).
- Degradación ya diseñada: si TimescaleDB cae, `ingest` sigue aceptando y el stream retiene el backlog (~1,4 GB/h a 1.000 msg/s) hasta drenarlo (ADR-0001, consecuencias).

Restricción dura: máquina dev macOS Intel con 16 GB RAM; orquestación local Docker Compose; cloud con Terraform. Prohibido Kubernetes/service mesh para este MVP.

## Decisión

**Monolito modular con costuras de eventos** (modular monolith con event-driven seams):

1. **Un módulo Go único** (`backend/go.mod`) con bounded contexts DDD (`telemetry`, `fleet`, `assistant`, `shared/domain`) y reglas de dependencia enforcement por CI — ya obligatorio por ADR-0002, cond. 1.
2. **Cuatro bins desplegables** desde ese módulo. **No son microservicios** porque:
   - No exponen APIs HTTP/RPC entre sí: su única integración es por eventos NATS durables y la DB compartida.
   - Comparten dominio, migrations y release train (todo se construye y versiona por commit).
   - Se escalan **replicando bins completos** (scale-out horizontal detrás de NATS), nunca fragmentando dominios.
3. **Los circuit breakers SÍ aplican y son obligatorios**, pero donde hay frontera de red real y fallo parcial posible — los bordes de E/S externa de cada bin, no entre módulos internos (llamadas a funciones en memoria, sin fallo parcial):

| Bin | Breaker (sony/gobreaker) | Comportamiento al abrirse |
|---|---|---|
| `ingest` | Publish a JetStream | Responder 503 + Retry-After; el móvil retiene el batch offline (su modo natural según el enunciado sec. 4.D) |
| `consumer` | Persistencia a TimescaleDB | Nak/redeliver con backoff acotado (MaxDeliver=3 → DLQ `telemetry.dlq`, ADR-0001 cond. 2); el backlog queda retenido en el stream |
| `api` | Queries de lectura (pool pgx) | Degradar SSE a último-valor-conocido/stale o 503 parcial; jamás bloquear goroutines del SSE esperando a la DB |
| `agent` | Llamada a Gemini/Vertex | Ya exigido por ADR-0003 cond. 5: respuesta inmediata de agente temporalmente no disponible; nunca esperar a un LLM colgado |

4. **Entre módulos internos no hay breaker**: no existe fallo parcial in-proceso; ahí el estándar es timeouts + colas acotadas (PublishAsyncMaxPending, MaxAckPending — ADR-0001 cond. 2).

## Opciones evaluadas y descartadas

| Opción | Veredicto | Motivo (para este workload y entorno) |
|---|---|---|
| **Microservicios clásicos** (N servicios con APIs propias, repos/despliegues/contratos independientes) | Descartada | Cada bin opera al 0,3-2 % del cuello de botella real (broker/disco): añadiría malla RPC, versionado de contratos entre servicios, tracing distribuido y N× overhead operativo **sin beneficio medible**; rompe la restricción operativa de 16 GB dev |
| **Un solo proceso que lo haga todo** (monolito de proceso único, 1 bin) | Descartada | El consumer (disco-bound) y el agent (latencia LLM variable, segundos) compartirían runtime con el SSE de lectura: un pico de GC del agent penalizaría las latencias del portal. Los 4 bins dan aislamiento real de recursos sin pagar el coste de microservicios |
| **Serverless/functions para la ingesta** | Descartada | Pierde el control de backpressure exigido (PublishAsyncComplete antes del 200 al dispositivo, ADR-0001 cond. 3); latencia/coste variables |

## Condiciones obligatorias

1. **Breaker + timeout en TODA E/S de red** de cada bin (tabla anterior). Prohibido invocar una dependencia de red sin ambos.
2. **Umbrales numéricos por dependencia, documentados en código/config**: p. ej. DB abre con ≥50 % de errores en ventana de 30 s o timeout >1 s sostenido; half-open tras 30 s con probe. Un breaker sin números calibrados es decorativo y se rechaza en review.
3. **Estados del breaker observables**: cambio de estado registrado (log métrico) y expuesto en `/healthz`/métricas Prometheus. Prohibido fallar en silencio.
4. **El write path nunca se bloquea por el read path ni por el LLM**: `ingest` depende exclusivamente de JetStream. Si DB, consumer o LLM caen, la ingesta sigue respondiendo 200 y el backlog se drena después. Todo refactor que introduzca una dependencia síncrona `ingest`→DB o `ingest`→LLM viola este ADR.
5. **Umbral de extracción a servicios separados** (extiende ADR-0001 cond. 7): >10.000 msg/s o >50.000 dispositivos → extraer/shardear el consumer (streams por shard + workers por `device_id`). Extraer el `agent` como servicio propio si su RSS supera 500 MB sostenidos o su cola de chat acumula >1.000 req pendientes. Hasta esos umbrales: réplicas de bins completos, nunca fragmentación de dominios.
6. **Enforcement de límites entre BCs en CI** (depguard/arch-lint, ya obligatorio por ADR-0002 cond. 1): sin él, el monolito modular degenera en big ball of mud y este ADR caduca. Es condición de merge desde el primer BC.
7. **Coherencia con ADR-0003**: el breaker del LLM, sus timeouts (~15 s) y rate limits (~10 req/min por usuario/IP) viven definidos allí; este ADR no los duplica ni relaja.
8. **Refrendo dedicado de `scalability` sobre este ADR** en cuanto el subagente esté disponible, y **bloqueante** antes de cerrar el SPEC de ingesta o antes de cruzar cualquier umbral de la condición 5. Los números heredados de ADR-0001/0002 cubren la decisión, pero el veredicto formal de esta síntesis queda pendiente.

## Consecuencias

**Positivas:**
- Se cumple el requisito de resiliencia del enunciado (circuit breakers ante fallos parciales) sin pagar el precio operativo de microservicios; la desviación del literal queda registrada y justificada para la sustentación.
- Degradación graceful verificable: caída de DB o broker no tumba el portal ni pierde telemetría (backlog en stream, batches offline en móvil, stale-read en SSE).
- Escalado simple: réplicas de bins completos consumiendo de colas durables — sin service discovery ni rebalanceo adicional.
- Coherente con la restricción de 16 GB (footprint vivo 6-9 GB) y con la regla de robustez de diseño sobre réplicas locales.

**Negativas:**
- El aislamiento de fallos es por proceso, no por dominio: una fuga de memoria en `consumer` afecta a sus réplicas (pero no a `api`/`ingest`).
- Exige disciplina continua: breakers calibrados + observabilidad de estados; uno mal calibrado puede enmascarar fallos reales (falso sano).
- Si se cruza el umbral de la condición 5, la extracción implica re-trabajo: migrar eventos internos compartidos a contratos versionados entre servicios.

## Referencias

- Dictámenes previos de `scalability` incorporados: ADR-0001 (condiciones 1, 2, 3, 7; sección de consecuencias) y ADR-0002 (condiciones 1, 7). Intento de dictamen dedicado 2026-08-22: subagente sin respuesta útil (×3); pendiente de refrendo (condición 8).
- ADR-0001 (JetStream backbone), ADR-0002 (módulo único + 4 bins), ADR-0003 (cond. 5: gobreaker sobre la llamada al LLM).
- `docs/PRUEBA-TECNICA.md` sec. 4.A (resiliencia) y sec. 4.D (offline-first batch sync).
- Stack decidido en AGENTS.md: `sony/gobreaker` para circuit breakers.
