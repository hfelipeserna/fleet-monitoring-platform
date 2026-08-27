---
name: expo-workflow
description: Usa esta skill al configurar y probar la app React Native Expo en dispositivo físico (Expo Go), con EAS Build, Fastlane y CI móvil. Trigger: expo, expo go, EAS, fastlane, mobile CI, EXPO_PUBLIC, LAN IP, tunnel, expo start, app.json, eas.json, eas build, preview, physical device.
---

# Expo workflow (Expo Go + EAS + Fastlane)

Principio: **el simulador miente; el cel físico dice la verdad**. PRUEBA-TECNICA sec 4.D exige CI/CD móvil y estrategia offline; el jurado espera probar la app escaneando QR en su celular. Todo lo que no funcione en Expo Go con LAN IP está roto.

## Expo Go en cel físico (DX)

- **Env var**: `EXPO_PUBLIC_API_URL` es la única vía pública. Expo solo inyecta `EXPO_PUBLIC_*` en JS bundle. En `lib/api.ts`:

```ts
const base = Constants.expoConfig?.extra?.apiUrl
  ?? process.env.EXPO_PUBLIC_API_URL
  ?? 'http://localhost:8080';
```

Hardcodear `localhost` sin LAN IP = **alta** (Expo Go en cel físico nunca alcanza `localhost` del laptop).

- **app.json**: fuente de verdad para Expo Go:

```json
{
  "expo": {
    "name": "fleet-mobile",
    "slug": "fleet-mobile",
    "extra": { "apiUrl": "http://LAN_IP:8080" },
    "ios": { "bundleIdentifier": "com.fleet.mobile" },
    "android": { "package": "com.fleet.mobile" }
  }
}
```

`extra.apiUrl` resuelve `localhost` vs LAN IP sin rebuild (puede sobreescribirse vía `eas.json` env).

- **LAN IP vs tunnel**: dos modos para Expo Go
  - `npx expo start` (LAN): rápido `p95 50ms`, requiere laptop+cel en misma WiFi. `EXPO_PUBLIC_API_URL=http://192.168.1.10:8080 npx expo start`.
  - `npx expo start --tunnel`: lento `p95 500ms` vía `ngrok`, útil si WiFi bloquea peer-to-peer pero no para demo offline (tunnel enmascara NetInfo). Documenta ambos en README con `ipconfig getifaddr en0` (macOS) / `ip addr` (Linux).
- **Comandos verificables**:

```sh
EXPO_PUBLIC_API_URL=http://192.168.1.10:8080 npx expo start        # LAN
EXPO_PUBLIC_API_URL=http://192.168.1.10:8080 npx expo start --tunnel # ngrok
npx expo-doctor   # healthcheck
npx tsc --noEmit --project mobile/tsconfig.json
```

- **Permisos**: `expo-location` solo para `ON->OFF` GPS real (no para simulada). Declarar en `app.json` `expo.plugins` con `expo-location` + `NSLocationWhenInUseUsageDescription` (iOS) + `ACCESS_FINE_LOCATION` (Android). Sin permiso, `Location.getCurrentPositionAsync` falla silencioso -> manejar `PermissionStatus`.

## EAS Build (cloud)

- **eas.json** (versionado, sin secretos):

```json
{
  "cli": { "version": ">= 5.9.0" },
  "build": {
    "development": { "developmentClient": true, "distribution": "internal" },
    "preview": { "distribution": "internal", "env": { "EXPO_PUBLIC_API_URL": "https://staging.example.com" } },
    "production": { "autoIncrement": true }
  },
  "submit": { "production": {} }
}
```

`development` para `expo-dev-client`, `preview` para QA vía link interno (APK/IPA), `production` con `autoIncrement`. Secretos reales via `eas secret:create` + `EAS_*` env, nunca en `eas.json`.

- **Comandos**:

```sh
npm i -g eas-cli
eas login
eas build --platform android --profile preview --local   # dry-run local (requiere Android SDK)
eas build --platform android --profile preview           # cloud (recomendado 16GB RAM)
eas build --platform ios --profile preview               # requiere Apple Team (opcional)
```

## Fastlane ( lanes )

- `fastlane/` con `Fastfile` + `Appfile`. Lanes mínimas para PRUEBA sec 4.D:

```ruby
# fastlane/Fastfile
lane :build_android do
  gradle(task: "assemble", build_type: "Release") # o eas via shell
end
lane :build_ios do
  gym(scheme: "fleetmobile") # bare only
end
```

Si usas `eas build` cloud, Fastlane es wrapper `sh("eas build --platform android --profile preview --non-interactive --no-wait")`. Documentar que Fastlane no hardcodea signing password; usa `MATCH_PASSWORD` via env.

## GitHub Actions móvil

- `.github/workflows/mobile.yml` con `path-filter`:

```yaml
on:
  push:
    paths: ["mobile/**", ".github/workflows/mobile.yml"]
  pull_request:
    paths: ["mobile/**"]
jobs:
  mobile:
    runs-on: ubuntu-latest
    defaults: { run: { working-directory: mobile } }
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: 20, cache: npm, cache-dependency-path: mobile/package-lock.json }
      - run: npm ci
      - run: npm run typecheck
      - run: npm run lint
      - run: npm test -- --run --coverage
      - run: npx expo export --platform all # valida bundle sin nativo
      # opcional: eas build cloud
      # - uses: expo/expo-github-action@v8
      #   with: { eas-version: latest, token: ${{ secrets.EXPO_TOKEN }} }
      # - run: eas build --platform android --profile preview --non-interactive --no-wait
```

No disparar `mobile` si solo cambia `backend/` (path-filter). Secretos `EXPO_TOKEN` via GitHub Secrets, nunca inline.

## Checklist DX antes de cerrar SPEC-005

- [ ] `mobile/.env.example` con `EXPO_PUBLIC_API_URL=http://localhost:8080` + comentario `LAN IP para Expo Go`.
- [ ] `README.md` sección `mobile/` con `EXPO_PUBLIC_API_URL=http://LAN_IP:8080 npx expo start` + `ipconfig` + QR.
- [ ] `npx expo start` abre Metro, `Expo Go` escanea QR y `POST /v1/telemetry 202` aparece en `docker logs ingest`.
- [ ] `npx expo-doctor` 0 issues, `tsc --noEmit` verde, `eas.json` valida `eas build --platform android --profile preview --local --dry-run` (si SDK disponible).
- [ ] `.github/workflows/mobile.yml` con `paths: mobile/**` y `EXPO_TOKEN` secreto.

## Verificación

- `docker compose config -q` intacto (LB `8080` único entry, no nuevo puerto móvil).
- `npx tsc --noEmit`, `npm test -- --run` >60% coverage, `npm run build` web intacto no regresa.
- Prueba física: modo avión en cel -> `Network ERROR` en app + cola crece; reconecta WiFi -> `Syncing CONNECTED` + `202` en logs.
