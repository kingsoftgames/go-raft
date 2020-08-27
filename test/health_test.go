package test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"google.golang.org/grpc"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
)

func Test_Health(t *testing.T) {
	genTestSingleYaml()
	singleAppTemplate(t, func() {
		ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
		con, err := grpc.DialContext(ctx, "127.0.0.1:18310", grpc.WithBlock(), grpc.WithInsecure())
		if err != nil {
			t.Errorf("Test_Health Failed,%s", err.Error())
		}
		c := healthgrpc.NewHealthClient(con)
		r, e := c.Check(ctx, &healthgrpc.HealthCheckRequest{Service: ""})
		if e != nil {
			t.Fatalf("Test_Health grpc err, %s", e.Error())
			return
		}
		t.Logf("Test_Health grpc status,%v", r.Status)

		if rsp, err := http.DefaultClient.Get("http://127.0.0.1:18320/health"); err != nil {
			t.Fatalf("Test_Health http err, %s", err.Error())
			return
		} else {
			t.Logf("Test_Health http status, %s", rsp.Status)
		}
	})
}
