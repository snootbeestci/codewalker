# Codewalker IntelliJ Plugin — Claude Code Briefing

This document captures all architectural decisions for the IntelliJ/PhpStorm
plugin. Do not re-litigate these decisions unless you encounter a hard
technical blocker. Ask the user before changing anything structural.

---

## What is this?

A PhpStorm (IntelliJ Platform) plugin that connects to a running Codewalker
gRPC backend and exposes its walkthrough and review session features directly
inside the IDE.

Target: PhpStorm. The plugin depends only on `com.intellij.modules.platform`
so it also works with IntelliJ IDEA CE/Ultimate and other IntelliJ-based IDEs.

---

## Repository layout

The plugin lives in the `plugin/` subdirectory of the main `codewalker` repo.

```
plugin/
├── build.gradle.kts           # IntelliJ Platform Gradle Plugin v2
├── settings.gradle.kts
├── gradlew / gradlew.bat
├── gradle/wrapper/
└── src/main/
    ├── kotlin/com/snootbeestci/codewalker/
    │   ├── grpc/
    │   │   ├── CodewalkerClient.kt          # Issue 3 — app-level gRPC service
    │   │   └── CodewalkerStartupActivity.kt # Issue 3 — startup connection
    │   └── settings/
    │       ├── CodewalkerSettings.kt         # Persistent settings (backendAddress)
    │       └── CodewalkerSettingsConfigurable.kt # Settings UI
    └── resources/META-INF/plugin.xml
```

---

## Tech stack

| Concern | Choice | Reason |
|---|---|---|
| IDE platform | IntelliJ Platform (PhpStorm target) | JetBrains plugin API |
| Build | IntelliJ Platform Gradle Plugin v2 (`org.jetbrains.intellij.platform`) | Current standard |
| Gradle version | 8.14.3 | Same wrapper as proto stubs project |
| Language | Kotlin 2.0.21 | JVM toolchain 17 |
| gRPC transport | `grpc-netty-shaded:1.68.1` | Avoids Netty class conflicts with IntelliJ bundled Netty |
| Proto stubs | `com.github.snootbeestci:codewalker-proto:0.1.9` (GitHub Packages) | Generated Kotlin stubs for the backend API |
| Settings storage | `PersistentStateComponent` + `@State` / `@Storage` | Standard IntelliJ persistence pattern |
| Coroutines | `kotlinx-coroutines-core:1.7.3` + `ApplicationManager.coroutineScope` | Platform-managed scope, available since IntelliJ 2023.2 |

---

## Architecture decisions

### Application-level gRPC service (`CodewalkerClient`)
- `CodewalkerClient` is an `@Service` (application-level) that owns the
  `ManagedChannel` and `CodeWalkerCoroutineStub`.
- It is the single owner of the channel. Nothing else creates channels.
- Registered in `plugin.xml` as `<applicationService>` so IntelliJ manages
  its lifecycle and calls `dispose()` on IDE shutdown.
- `getStub()` returns `null` when disconnected; callers must check.

### Connection state machine
- Three states: `DISCONNECTED`, `INCOMPATIBLE`, `CONNECTED`.
- `connect(address)` always tears down the previous channel first, then
  attempts a new connection and calls `GetVersion` to check `proto_major`.
- `SUPPORTED_PROTO_MAJOR = 1` — increment when the plugin requires a new
  breaking proto change.
- Every state transition from `connect()` publishes to `CONNECTION_STATE_TOPIC`
  (even failures). `disconnect()` does NOT publish — it is an internal
  teardown step, not a user-visible event.
- Subscribers implement `ConnectionStateListener` and register via the
  IntelliJ message bus.

### Settings persistence
- `CodewalkerSettings` stores `backendAddress` (default: `localhost:50051`).
- Persisted to `codewalker.xml` in the IDE config directory.
- UI in `CodewalkerSettingsConfigurable` (Tools > Codewalker settings panel).
- Changing and applying settings triggers reconnection via
  `ApplicationManager.getApplication().coroutineScope.launch { ... }`.

### Startup connection
- `CodewalkerStartupActivity` implements `ProjectActivity` (not the older
  `StartupActivity`). It calls `connect()` once per project open with the
  currently configured address.
- Because `CodewalkerClient` is application-level, multiple project windows
  share one channel. Successive calls to `connect()` are safe — each tears
  down the previous channel first.

### gRPC transport
- Uses `grpc-netty-shaded` to avoid class conflicts with IntelliJ's bundled
  Netty. Never use `grpc-netty` (unshaded) in a plugin.
- `usePlaintext()` only — TLS is not in scope for v1. Backend runs on
  localhost in Docker.

---

## Proto stub coordinates

```
com.github.snootbeestci:codewalker-proto:0.1.9
```

Published to GitHub Packages. Build requires `GITHUB_ACTOR` and `GITHUB_TOKEN`
environment variables (read-only scope is sufficient).

The generated Kotlin coroutine stub class is:
```
codewalker.v1.CodeWalkerGrpcKt.CodeWalkerCoroutineStub
```

If the build fails to resolve this class, check the generated sources in the
Gradle cache to confirm the exact package and class names before proceeding.

---

## Build

```bash
cd plugin
./gradlew buildPlugin   # compiles and packages the plugin zip
./gradlew check         # runs verifyPlugin + compileKotlin checks
```

The IntelliJ Platform Gradle Plugin v2 downloads IntelliJ IDEA Community
2024.3 on first build. This is large (~1 GB) — expect a slow first build.

Target IDE for CI is IntelliJ IDEA Community. Production target is PhpStorm,
which is based on the same platform build and is ABI-compatible for plugins
depending only on `com.intellij.modules.platform`.

---

## Development rules

- New plugin features go in `plugin/src/main/kotlin/com/snootbeestci/codewalker/`
- Never register the same extension point twice in `plugin.xml`
- `getStub()` returns null when disconnected — every call site must handle null
- Never store `forge_token` or any secrets in plugin settings
- `./gradlew check` must pass before any PR targeting plugin code
- When adding a new application-level service, register it in `plugin.xml`
  under `<applicationService>` AND annotate the class with `@Service`
- Use `ApplicationManager.getApplication().coroutineScope` for launching
  coroutines from non-suspend contexts inside the plugin (available since
  IntelliJ 2023.2 / build 232)

---

## Issue history

| Issue | Title | Status |
|---|---|---|
| 1 | Plugin scaffold + settings persistence | Completed (setup branch) |
| 2 | Settings UI (CodewalkerSettingsConfigurable) | Completed (setup branch) |
| 3 | gRPC client and connection state | Completed (setup branch) |
