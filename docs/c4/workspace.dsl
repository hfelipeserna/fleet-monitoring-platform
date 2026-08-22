workspace "Fleet Monitoring Platform" "Portal Corporativo de Monitoreo de Flotas - C4 Model" {

    model {
        operador = person "Operador de Flota" "Hace uso del portal web (SPA)" "Person"
        conductor = person "Conductor" "Hace uso de la aplicación móvil" "Person"

        fleetPlatform = softwareSystem "Fleet monitoring platform" "Ingesta de datos, integración con IA y dashboard" "System"

        gemini = softwareSystem "Gemini API" "IA generativa" "External"

        operador -> fleetPlatform "Consulta dashboard y chat con IA" "HTTPS - portal web"
        fleetPlatform -> operador "Alertas y datos en tiempo real" "SSE - /api/alerts"
        conductor -> fleetPlatform "Envía telemetría GPS" "HTTPS - batch offline-first"
        fleetPlatform -> gemini "Consultas NL" "HTTPS - Gemini API via Genkit"
    }

    views {
        systemContext fleetPlatform "Contexto_N1" {
            include *
            autoLayout lr 400 250
            title "C4 Nivel 1 - Contexto - Fleet Monitoring Platform"
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
            relationship "Relationship" {
                thickness 2
                colour #374151
                routing Direct
                position 50
            }
        }
    }

}
