package service

import (
    "encoding/json"
    "net"
    "testing"

    "github.com/mhsanaei/3x-ui/v3/internal/database"
    "github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func testXHTTPManagedStream(port int, listen string, enabled bool, security string) string {
    cfg := map[string]any{
        "network":"xhttp", "security":"tls",
        "xhttpSettings":map[string]any{
            "downloadSettings":map[string]any{
                "port":port, "security":security,
                "managedListener":map[string]any{"enabled":enabled,"listen":listen},
            },
        },
    }
    b,_:=json.Marshal(cfg)
    return string(b)
}
func testXHTTPManagedInbound(port int, down int, listen string) *model.Inbound {
    return &model.Inbound{
        Remark:"test", Tag:"test-candidate", Listen:"0.0.0.0", Port:port, Protocol:model.VLESS,
        StreamSettings:testXHTTPManagedStream(down,listen,true,"reality"),
    }
}
func TestXHTTPManagedPortCollisionAgainstExistingMain(t *testing.T) {
    setupConflictDB(t)
    seedInboundConflict(t,"existing-443","0.0.0.0",443,model.VLESS,`{"network":"xhttp"}`, "{}")
    got,err:=(&InboundService{}).checkPortConflict(testXHTTPManagedInbound(2083,443,"0.0.0.0"),0)
    if err!=nil || got==nil || got.Port!=443 {
        t.Fatalf("managed 443 should conflict with existing main 443: %v %v",got,err)
    }
}
func TestXHTTPManagedPortCollisionAgainstExistingManaged(t *testing.T) {
    setupConflictDB(t)
    seedInboundConflict(t,"existing-managed","0.0.0.0",2083,model.VLESS,testXHTTPManagedStream(9443,"0.0.0.0",true,"reality"),"{}")
    svc:=&InboundService{}
    // Another managed listener must not reserve the same port.
    got,err:=svc.checkPortConflict(testXHTTPManagedInbound(2053,9443,"0.0.0.0"),0)
    if err!=nil || got==nil || got.Port!=9443 {t.Fatalf("managed/managed not detected: %v %v",got,err)}
    // Even a plain inbound must not take someone else's managed port.
    ordinary:=&model.Inbound{Tag:"ordinary",Listen:"0.0.0.0",Port:9443,Protocol:model.Trojan}
    got,err=svc.checkPortConflict(ordinary,0)
    if err!=nil || got==nil || got.Port!=9443 {t.Fatalf("ordinary/managed not detected: %v %v",got,err)}
}
func TestXHTTPManagedPortSelfConflict(t *testing.T) {
    setupConflictDB(t)
    got,err:=(&InboundService{}).checkPortConflict(testXHTTPManagedInbound(443,443,"0.0.0.0"),0)
    if err!=nil || got==nil || got.Port!=443 {t.Fatalf("same uplink/downlink port not detected: %v %v",got,err)}
}
func TestXHTTPManagedPortTLSNoExtraSocket(t *testing.T) {
    setupConflictDB(t)
    seedInboundConflict(t,"existing-9443","0.0.0.0",9443,model.VLESS,`{"network":"xhttp"}`,"{}")
    incoming:=testXHTTPManagedInbound(2083,9443,"0.0.0.0")
    incoming.StreamSettings=testXHTTPManagedStream(9443,"0.0.0.0",false,"tls")
    got,err:=(&InboundService{}).checkPortConflict(incoming,0)
    if err!=nil || got!=nil {t.Fatalf("TLS download via CDN must not reserve a local listener: %v %v",got,err)}
}
func TestXHTTPManagedPortAllowsUDPCoexist(t *testing.T) {
    setupConflictDB(t)
    seedInboundConflict(t,"udp-9443","0.0.0.0",9443,model.Hysteria,"","")
    got,err:=(&InboundService{}).checkPortConflict(testXHTTPManagedInbound(2083,9443,"0.0.0.0"),0)
    if err!=nil || got!=nil {t.Fatalf("TCP REALITY should coexist with UDP: %v %v",got,err)}
}
func TestXHTTPManagedPortExternalBind(t *testing.T) {
    setupConflictDB(t)
    listener,err:=net.Listen("tcp","127.0.0.1:0")
    if err!=nil {t.Fatal(err)}
    defer listener.Close()
    port:=listener.Addr().(*net.TCPAddr).Port
    if port==2083 {t.Skip("ephemeral port happens to be uplink")}
    incoming:=testXHTTPManagedInbound(2083,port,"127.0.0.1")
    got,err:=(&InboundService{}).checkPortConflict(incoming,0)
    if err==nil || got!=nil {t.Fatalf("external OS socket conflict not detected: %v %v",got,err)}
}
func TestXHTTPManagedPortEditingOwnActivePort(t *testing.T) {
    setupConflictDB(t)
    listener,err:=net.Listen("tcp","127.0.0.1:0")
    if err!=nil {t.Fatal(err)}
    defer listener.Close()
    port:=listener.Addr().(*net.TCPAddr).Port
    if port==2083 {t.Skip("ephemeral port happens to be uplink")}
    old:=testXHTTPManagedInbound(2083,port,"127.0.0.1")
    old.Enable=true
    if err:=database.GetDB().Create(old).Error;err!=nil {t.Fatal(err)}
    updated:=*old
    got,err:=(&InboundService{}).checkPortConflict(&updated,old.Id)
    if err!=nil || got!=nil {t.Fatalf("unchanged live managed port must be editable: %v %v",got,err)}
}
