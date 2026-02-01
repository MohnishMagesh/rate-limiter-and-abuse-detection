### start redis
docker run -d -p 6379:6379 redis:alpine
docker ps -a
docker stop <container_id>
docker rm <container_id>

go run cmd/server/main.go
go run cmd/client/main.go

### after adding docker
docker compose up --build
docker compose down


#### Key Notes
Rate limiting is done by User ID and action
Example:- Redis Key: rate_limit:User_123:login 


```mermaid
graph LR
    subgraph "Client"
        User["End User / Mobile App"]
    end

    subgraph "Backend Infra"
        direction TB
        MainAPI["Main Backend API<br/>The Client"]

        subgraph "Your Project - The Rate Limiter"
            RL_Service["Rate Limiter Service<br/>Go + gRPC Server"]
            Redis[("Redis Cache<br/>+ Lua Scripts")]
        end
    end

    %% Flows
    User -- "1. Login Request (HTTP)" --> MainAPI
    MainAPI -- "2. Check Limit (gRPC)" --> RL_Service
    RL_Service -- "3. Execute Script (Lua)" --> Redis
    Redis -- "4. Returns Count" --> RL_Service
    RL_Service -- "5. Allow / Deny" --> MainAPI

    %% Decision path
    MainAPI -- "6a. Success (200 OK)" --> User
    MainAPI -- "6b. Blocked (429 Error)" --> User

    %% Styling
    style User fill:#f9f,stroke:#333,stroke-width:2px
    style MainAPI fill:#bbf,stroke:#333,stroke-width:2px
    style RL_Service fill:#bfb,stroke:#333,stroke-width:2px
    style Redis fill:#fbb,stroke:#333,stroke-width:2px

```

### Advanced improvements


Dynamic config reload --> Change system rules without restarting the server (use watcher on config file or background goroutine checking redis every 10 seconds for new rules)

Shadow mode --> allow users for new version but log errors and potential rate limits/blocking of requests (can't afford to push these down in production)

Adaptive throttling --> (low latency - limit can be increased, system suffering from high latency - limit should be decreased)


Hot-path optimized --> "Optimized" means you have removed every millisecond of wasted time. 