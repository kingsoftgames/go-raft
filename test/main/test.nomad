#
# Manual test service 
#
job "test" {
  datacenters = ["[[.datacenter]]"]
  type = "service"

  // Enable rolling updates
  update {
    max_parallel = 1
  }

  group "test" {
    count = [[ .count ]]
    restart {
      delay = "15s"
      mode = "fail"
    }
    task "test" {
      driver = "exec"
      config {
        command = "test_main"
        args = ["--config=local/config.yml"]
      }
      artifact {
        source = "[[ $.artifact ]]"
      }
      kill_timeout = "30s"
      resources {
        memory = 2000
        network {
          mbits = 10
          port "raft" {}
          port "grpc" {}
          port "http" {}
          port "inner" {}
        }
      }
      service {
        name = "test-grpc"
        tags = [
          "internal-/test.Test proto=grpc", // For fabio, internal-facing
        ]
        port = "grpc"
        address_mode = "host"
        check {
          name     = "Test grpc check"
          type     = "grpc"
          interval = "1s"
          timeout  = "2s"
        }
      }
      service {
        name = "test-inner-grpc"
        tags = [
          "internal-/inner.Raft proto=grpc", // For fabio, internal-facing
        ]
        port = "inner"
        address_mode = "host"
        check {
          name     = "Test-inner grpc check"
          type     = "grpc"
          interval = "1s"
          timeout  = "2s"
        }
        check_restart {
          limit    = 3
          grace    = "20s"
        }
      }
      service {
        name = "test"
        tags = [
          "internal-/test", // For fabio, internal-facing
        ]
        port = "http"
        address_mode = "host"
        check {
          name     = "Test http check"
          type     = "http"
          path     = "/health"
          interval = "1s"
          timeout  = "2s"
          }
      }
      template {
        data = <<EOF
{{ range service "test-inner-grpc" }}
{{ .Address }}:{{ .Port }}{{ end }}
EOF
        change_mode = "noop"
        destination = "local/join_addr.txt"
      }
      template {
        data = <<EOF
raft_addr: {{ env "NOMAD_IP_raft" }}:{{ env "NOMAD_PORT_raft" }}
grpc_addr: {{ env "NOMAD_IP_grpc" }}:{{ env "NOMAD_PORT_grpc" }}
http_addr: {{ env "NOMAD_IP_http" }}:{{ env "NOMAD_PORT_http" }}
inner_addr: {{ env "NOMAD_IP_inner" }}:{{ env "NOMAD_PORT_inner" }}
store_dir: local/cache
store_in_mem:  true
codec: msgpack
node_id: test-node-{{ env "NOMAD_ALLOC_INDEX" }}
join_file: local/join_addr.txt
try_join_time: 3
connect_timeout_ms: 5000
bootstrap: false
bootstrap_expect: [[ .count ]]
log_config:
  level: DEBUG
  path: local/log
  max_age: 15
  max_size: 100
EOF
        destination = "local/config.yml"
      }
    }
  }
}
