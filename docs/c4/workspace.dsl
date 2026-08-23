workspace "Fleet Monitoring Platform" "Portal Corporativo de Monitoreo de Flotas - C4 Model" {

    model {
        operador = person "Operador de Flota" "Hace uso del portal web (SPA)" "Person"
        conductor = person "Conductor" "Hace uso de la aplicación móvil" "Person"

        fleetPlatform = softwareSystem "Fleet monitoring platform" "Ingesta de datos, integración con IA y dashboard" "System" {
            mobile = container "Mobile App" "Offline-first: Captura telemetría GPS y sincroniza en bloque" "React Native + WatermelonDB"
            web = container "Web Application" "Mapa en tiempo real, alertas SSE y chat con la IA (built con Vite)" "React + Leaflet/MapLibre"
            lb = container "Load Balancer" "L7 - healthcheck, drain y round-robin" "nginx:alpine / ALB"
            ingest = container "Telemetry Ingest API" "Valida y publica telemetría entrante como eventos" "Go (net/http)"
            nats = container "Event Broker" "Streams durables de telemetrías y alertas" "NATS JetStream"
            consumer = container "Consumer Worker" "Consume el stream y persiste series de tiempo" "Go (nats.go, pgx)"
            db = container "TimescaleDB" "Hipertables con datos de telemetría" "PostgreSQL+Timescale"
            api = container "Platform API" "HTTP/SSE: telemetría agregada, alertas, circuit breakers" "Go (Clean architecture)"
            agent = container "AI Agent" "Flows y tools de consulta al estado de la flota" "Go + Genkit"

            # Relaciones alineadas 100% a tu diagrama + LB
            mobile -> lb "Sincroniza en bloque" "HTTPS - batch offline-first"
            lb -> ingest "Sincroniza en bloque" "HTTPS"
            ingest -> nats "Publica eventos de telemetría [NATS]" "NATS"
            consumer -> nats "Consume streams durable [NATS]" "NATS"
            consumer -> db "Escribe series de tiempo (SQL)" "SQL"
            web -> api "Consulta telemetría y alertas" "HTTPS/SSE"
            web -> agent "Chat en lenguaje Natural" "HTTPS"
            api -> db "Lee telemetría agregada" "SQL"
            agent -> db "Tools consultan estado de la flota (SQL)" "SQL"
        }

        gemini = softwareSystem "Gemini API" "IA generativa" "External"

        # Relaciones Nivel 1 (Contexto)
        operador -> fleetPlatform "Consulta dashboard y chat con IA" "HTTPS - portal web"
        fleetPlatform -> operador "Alertas y datos en tiempo real" "SSE - /api/alerts"
        conductor -> fleetPlatform "Envía telemetría GPS" "HTTPS - batch offline-first"
        fleetPlatform -> gemini "Consultas NL" "HTTPS - Gemini API via Genkit"

        # Relaciones Nivel 2 personas->contenedores (como en tu imagen)
        operador -> web "Visualiza dashboard, alertas y chat con IA" "HTTPS"
        conductor -> mobile "Envía telemetría GPS" "HTTPS"
        agent -> gemini "Consultas NL" "HTTPS - Gemini API via Genkit"
    }

    views {
        systemContext fleetPlatform "Contexto_N1" {
            include *
            autoLayout lr 400 250
            title "C4 Nivel 1 - Contexto - Fleet Monitoring Platform"
        }

        container fleetPlatform "Contenedores_N2" {
            include *
            autoLayout tb 400 250
            title "C4 Nivel 2 - Contenedores - Fleet Monitoring Platform"
            description "Fleet monitoring platform - Nivel 2. Monolito modular + LB + NATS + TimescaleDB. Ver ADR-0001/0002/0005/0006/0007."
        }

        theme default

        styles {
            element "Person" {
                background #1e40af
                colour #ffffff
                shape Person
            }
            element "System" {
                background #3b82f6
                colour #ffffff
            }
            element "External" {
                background #6b7280
                colour #ffffff
            }
            element "Container" {
                background #ffffff
                colour #000000
            }
            relationship "Relationship" {
                thickness 2
                colour #374151
                routing Direct
                position 50
            }
        }
    }

}
