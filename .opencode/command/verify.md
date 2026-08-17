---
description: Corre todas las verificaciones de "done": build/vet/tests Go, build/lint web y móvil, y docker compose config.
agent: architect
---

Ejecuta la batería de verificación del proyecto para confirmar definición de "done" (scope: $ARGUMENTS si se indica, si no todo):

1. Go: `go build ./...` y `go vet ./...` en el module root del backend; si hay tests, `go test ./...`.
2. Web: `npm ci` (o install) + `npm run build` y `npm run lint` en el frontend.
3. Móvil: `npm run typecheck` y `npm run lint` en la app Expo.
4. Infra: `docker compose config -q` (o `config`) si existe compose; `terraform fmt -check` + `terraform validate` en los directorios terraform.
5. Reporta por componente: ✔/✘ con el comando y el output relevante. Si algo falla, abre el fix con el especialista correspondiente y re-corre.
6. No cambies de scope sin avisarme.