module github.com/ethoDomingues/braza

go 1.20

replace github.com/ethoDomingues/c3po => ../c3po

require (
	github.com/ethoDomingues/c3po v0.0.0-00010101000000-000000000000
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/joho/godotenv v1.5.1
	gopkg.in/yaml.v2 v2.4.0
)
