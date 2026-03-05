# NetControl-Plane

NetControl-Plane - это сервис для управления сетевым доступом через декларативные intents

## Управление политиками доступа

- описывание намерений (intent)
- ревизии
- построение план изменений
- применение в Kubernetes
- отслеживание drift, audit и TTL-истечение


## Достоинства

- Четкое разделение слоев (`domain`, `app`, `ports`, `adapters`)
- Жизненный цикл `intent -> revision -> plan -> apply`
- audit trail для операций
- TTL-сценарии: auto-expire, delete и rollback
- idempotency-поведение для apply
- OpenAPI и e2e сценарии для демонстрации

## Быстрый запуск демо

1. `cd ncp`
2. `make env-copy`
3. `make demo`

`make demo` поднимает БД, запускает API/worker, выполняет e2e сценарии и выводит сводку по планам, джобам и аудиту.

