# Take-Home Assessment: Senior Fullstack Engineer

> **Fuente canónica de requerimientos.** Todo SPEC en `docs/specs/` deriva de este documento.
> Guardado verbatim. Cualquier duda de alcance se resuelve contra este texto.

---

## Telemetría y Desarrollo Agéntico Extremo

¡Hola! Bienvenido/a a nuestro proceso de selección. En esta etapa, buscamos evaluar tus capacidades como desarrollador Senior Fullstack de élite. Para este nivel, esperamos que vayas mucho más allá de la construcción de APIs CRUD y el consumo de interfaces.

## 1. Contexto y Filosofía de la Prueba

Esta prueba tiene una carga de trabajo intencionalmente alta porque evaluamos a ingenieros modernos bajo metodologías de entrega rápida (Fast Track). Asumimos de forma explícita que utilizarás IDEs agénticos (Cursor, Windsurf, GitHub Copilot Workspace, Antigravity) para acelerar tu desarrollo. No queremos que piques código repetitivo a mano. Queremos evaluar tu capacidad como Arquitecto y Senior Developer para orquestar la IA, guiarla hacia los más altos estándares internacionales (Clean Architecture, DDD, SOLID) y entregar un ecosistema complejo, resiliente y de alta calidad en una fracción del tiempo tradicional.

## 2. Transparencia y Propiedad Intelectual

Sabemos que en la industria existen malas prácticas donde las pruebas técnicas se acercan al trabajo corporativo no remunerado. Queremos ser absolutamente claros:

- **Este código es tuyo**: Te pedimos que alojes este desafío en un repositorio público (GitHub/GitLab) en tu cuenta personal. Eres libre de usarlo para tu portafolio.
- **No es código de producción**: El escenario de telemetría planteado está abstraído y simplificado. No usaremos ni integraremos ninguna línea de código de tu prueba en nuestra plataforma.
- **Evaluamos el "Cómo", no el "Qué"**: Dado que te pedimos usar IA para acelerar el desarrollo, nuestro enfoque de evaluación está en tus decisiones arquitectónicas, tu auditoría de la herramienta agéntica y tu diseño de infraestructura, no en obtener un MVP gratuito.

## 3. Contexto Tecnológico

Nuestra plataforma base maneja volúmenes masivos de datos y opera bajo los siguientes lineamientos técnicos:

| Capa Técnica | Tecnologías Core |
|---|---|
| Frontend Web | React, NextJS |
| Mobile | React Native |
| Backend | Go, C# (.NET) |
| Ingesta y Eventos | Kafka |
| Persistencia | TimescaleDB, Cassandra (Alta frecuencia / Series de tiempo) |
| Analítica | Druid |
| Infraestructura (Cloud) | AWS (Control Tower, Multi-cuenta) |

> Nota: Eres libre de elegir tu propio stack para esta prueba. Nos enfocaremos en tus decisiones arquitectónicas demostrando que la IA funciona como tu exoesqueleto y no como una muleta.

## 4. El Desafío: Portal Corporativo y Telemetría

Tu misión es diseñar y desarrollar un MVP funcional de un Portal Corporativo para el Monitoreo de Flotas, integrando un pipeline de telemetría de alta concurrencia, capacidades de IA, y una aplicación móvil para la captura de datos en campo.

### A. Arquitectura de Ingesta Orientada a Eventos (Backend)

- **Stream de Datos**: Implementar un flujo de ingesta asíncrona usando un bus de eventos (ej. Kafka, RabbitMQ) para procesamiento desacoplado.
- **Persistencia Especializada**: Diseñar y justificar la capa de almacenamiento pensando en miles de dispositivos (ej. bases de datos de series de tiempo como TimescaleDB).
- **Resiliencia**: Implementar Circuit Breakers en la comunicación entre microservicios.

### B. Desarrollo Agéntico (IA Integrada)

- **Agente Operativo**: Implementar un agente (LangChain, Semantic Kernel, etc.) integrado en tu backend que consuma el estado actual de los vehículos y responda consultas en lenguaje natural (ej. "¿Qué vehículos llevan detenidos más de 20 minutos en zonas críticas?").

### C. Portal Corporativo (Frontend Web)

- **Dashboard Reactivo**: SPA que consuma los datos procesados mediante WebSockets o SSE (Server-Sent Events). Mostrar mapa, alertas en tiempo real y chat con la IA.

### D. Ecosistema Móvil (App del Conductor)

- **Offline-First**: App móvil (React Native u otro) que envíe coordenadas. Debe persistir localmente (SQLite, WatermelonDB) si se pierde la red y sincronizar en bloque al reconectar.
- **CI/CD Móvil**: Incluir archivos de automatización de despliegue (Fastlane, GitHub Actions).

### E. Infraestructura, Caos y Testing

- **Caos y Carga**: Script (k6, JMeter) para simular cientos de vehículos, inyectando 10% de peticiones duplicadas y 5% de errores.
- **Cloud & Docker**: Proveer scripts de IaC (Terraform, AWS CDK) y ejecución local orquestada vía Docker Compose.

## 5. Expectativa de Tiempo y Recursos

Estimamos que un perfil Senior apoyado correctamente en herramientas de Inteligencia Artificial agéntica puede orquestar este ecosistema y preparar sus entregables en 8 a 12 horas efectivas de trabajo.

**Gestión de Recursos**: Si tu entorno local no soporta levantar todos los contenedores simultáneamente (Kafka, Bases de datos, Frontend, Backend, etc.), esperamos que uses tu criterio ejecutivo. Es completamente válido utilizar mocks ligeros, apalancarte en capas gratuitas de la nube (Free Tiers), y justificar estas decisiones arquitectónicas durante tu sustentación.

## 6. Criterios de Evaluación

| Dimensión | Expectativa Nivel Senior |
|---|---|
| Arquitectura Agéntica | Capacidad de auditar y refactorizar el código generado por la IA para cumplir con estándares Clean Architecture y resiliencia. |
| Manejo de Datos | Uso correcto de eventos (Kafka) y justificación de la base de datos para alta frecuencia. |
| Mobile & Edge | Estrategia Offline-First sólida y automatización de despliegues (CI/CD). |

## 7. Entregables y Auditoría de IA

1. **Repositorio Git**: Con historial de commits estructurado.
2. **Documentación (README.md)**: Instrucciones de ejecución e IaC.
3. **Auditoría de IA (Fundamental)**: Incluida en tu README. Detalla al menos dos decisiones arquitectónicas o de código donde el IDE agéntico sugirió un enfoque deficiente, inseguro o no escalable, y cómo aplicaste tu criterio para forzar a la IA a refactorizar hacia un estándar internacional.
4. **Video de Sustentación Virtual** (5 a 10 min, YouTube No Listado):
   - Explicación de la arquitectura y demostración funcional.
   - **Demostración de Aceleración**: Dedica 2 minutos a mostrar cómo estructuraste tu entorno de trabajo agéntico (ej. configuración de .cursorrules, definición de contexto/prompts).
