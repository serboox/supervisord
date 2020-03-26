## Supervisord

> Supervisord is a server application with which the user can control connected processes on UNIX systems.

#### Application configuration example
```yaml
programs:
  - program_name: http-server
    command: "go run main.go"
    directory: "./http-server"
    autostart: True
    autorestart: True
    startsecs: 1
    restartpause: 0
    startretries: 3
    stopsignal: KILL
    stopwaitsecs: 10
    user: sergey:sergey
    environment:
      - name: GOROOT
        value: "/usr/local/go"
    health:
      scheme: http
      host: 127.0.0.1
      port: 8077
      path: "/healthcheck"
      attempts: 2
      attempts_delay_milliseconds: 5000
```