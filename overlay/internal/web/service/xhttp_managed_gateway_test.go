package service

import (
 "encoding/json"
 "strings"
 "testing"

 "github.com/mhsanaei/3x-ui/v3/internal/database/model"
 "github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
 "github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func mockManagedDown() map[string]any {
 return map[string]any{
  "port":float64(9443),"security":"reality",
  "realitySettings":map[string]any{"serverName":"www.example.com","publicKey":"PUBLIC","shortId":"0a"},
  "managedListener":map[string]any{
   "enabled":true,"listen":"0.0.0.0",
   "realitySettings":map[string]any{
    "target":"www.example.com:443",
    "serverNames":[]any{"www.example.com"},
    "shortIds":[]any{"0a"},
    "privateKey":"PRIVATE","publicKey":"PUBLIC",
   },
  },
 }
}
func mockTLSXHTTP(t *testing.T) xray.InboundConfig {
 t.Helper()
 return xray.InboundConfig{
  Port:443,Tag:"inbound-11",Protocol:"vless",
  Listen:json_util.RawMessage(`"0.0.0.0"`),
  Settings:json_util.RawMessage(`{"clients":[{"id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","email":"user@example.test"}],"decryption":"none"}`),
  StreamSettings:json_util.RawMessage(`{"network":"xhttp","security":"tls","xhttpSettings":{"path":"/share"},"tlsSettings":{"serverName":"cdn.example.com","alpn":["h2"],"certificates":[{"certificateFile":"/cert","keyFile":"/key"}]}}`),
 }
}
func TestXHTTPManagedGatewaySharedFallback(t *testing.T) {
 base:=mockTLSXHTTP(t)
 gateways,err:=xhttpGatewayConfig(base,11,mockManagedDown(),map[int]string{443:base.Tag})
 if err!=nil {t.Fatal(err)}
 if len(gateways)!=3 {t.Fatalf("expected 3 generated listeners, got %d",len(gateways))}
 internal,tls,reality:=gateways[0],gateways[1],gateways[2]
 if internal.Tag!=base.Tag || internal.Port!=0 {t.Fatal("shared listener must own original stats tag")}
 if !strings.Contains(string(internal.Listen),"@x3p-xhttp-11") {t.Fatal("internal listener must use private abstract socket")}
 inner:=xhttpMap(internal.StreamSettings)
 if inner["security"]!="none" || inner["network"]!="xhttp" {t.Fatalf("invalid inner stream %#v",inner)}
 if _,ok:=inner["tlsSettings"];ok {t.Fatal("inner must not require TLS")}
 if tls.Port!=443 || reality.Port!=9443 {t.Fatalf("ports wrong %d %d",tls.Port,reality.Port)}
 tlsStream:=xhttpMap(tls.StreamSettings)
 realStream:=xhttpMap(reality.StreamSettings)
 if tlsStream["network"]!="raw" || tlsStream["security"]!="tls" {t.Fatalf("incorrect TLS front %#v",tlsStream)}
 if realStream["network"]!="raw" || realStream["security"]!="reality" {t.Fatalf("incorrect REALITY front %#v",realStream)}
 realityParams,ok:=realStream["realitySettings"].(map[string]any)
 if !ok || realityParams["minClientVer"]!="1.8.2" {
   t.Fatalf("managed REALITY listener must permit Mihomo's 1.8.2 handshake: %#v",realStream)
 }
 for _,front:=range gateways[1:] {
  settings:=xhttpMap(front.Settings)
  if len(settings["clients"].([]any))!=0 {t.Fatal("gateway must not duplicate XHTTP users")}
  fallback:=settings["fallbacks"].([]any)[0].(map[string]any)
  if fallback["dest"]!="@x3p-xhttp-11" || fallback["xver"]!=float64(0) && fallback["xver"]!=0 {t.Fatalf("bad fallback %#v",fallback)}
 }
 settings:=xhttpMap(internal.Settings)
 if len(settings["clients"].([]any))!=1 {t.Fatal("inner must retain clients and original traffic accounting")}
 if strings.Contains(string(reality.StreamSettings),"PUBLIC") {t.Fatal("public client key must not appear in server REALITY stream")}
}
func TestXHTTPManagedGatewayValidation(t *testing.T) {
 base:=mockTLSXHTTP(t)
 for _,test:=range []struct{name string; mutate func(map[string]any); err string}{
  {"port conflict",func(d map[string]any){d["port"]=float64(443)},"conflicts"},
  {"missing private key",func(d map[string]any){d["managedListener"].(map[string]any)["realitySettings"].(map[string]any)["privateKey"]=""},"incomplete"},
  {"wrong public key",func(d map[string]any){d["realitySettings"].(map[string]any)["publicKey"]="UNRELATED"},"differ"},
  {"wrong SNI",func(d map[string]any){d["realitySettings"].(map[string]any)["serverName"]="wrong.example.com"},"not allowed"},
  {"wrong short id",func(d map[string]any){d["realitySettings"].(map[string]any)["shortId"]="ffff"},"not allowed"},
 } {
  t.Run(test.name,func(t *testing.T){
   d:=mockManagedDown()
   test.mutate(d)
   _,err:=xhttpGatewayConfig(base,11,d,map[int]string{443:base.Tag})
   if err==nil || !strings.Contains(err.Error(),test.err) {t.Fatalf("expected %q, got %v",test.err,err)}
  })
 }
}
func TestXHTTPManagedGatewayExplicitOptIn(t *testing.T) {
 base:=mockTLSXHTTP(t)
 xhttp:=map[string]any{"downloadSettings":mockManagedDown()}
 stream:=map[string]any{"network":"xhttp","security":"tls","xhttpSettings":xhttp}
 raw,_:=json.Marshal(stream)
 row:=&model.Inbound{Id:11,Tag:base.Tag,Enable:true,Protocol:model.VLESS,StreamSettings:string(raw)}
 cfg:=&xray.Config{InboundConfigs:[]xray.InboundConfig{base}}
 if err:=injectManagedXHTTPGateways(cfg,[]*model.Inbound{row},map[string]map[string]any{base.Tag:mockManagedDown()});err!=nil {t.Fatal(err)}
 if len(cfg.InboundConfigs)!=3 {t.Fatalf("expected 3 generated endpoints, got %d",len(cfg.InboundConfigs))}
 // disabling metadata leaves original TLS/XHTTP endpoint untouched
 xhttp["downloadSettings"].(map[string]any)["managedListener"].(map[string]any)["enabled"]=false
 raw,_=json.Marshal(stream);row.StreamSettings=string(raw)
 cfg=&xray.Config{InboundConfigs:[]xray.InboundConfig{base}}
 if err:=injectManagedXHTTPGateways(cfg,[]*model.Inbound{row},map[string]map[string]any{base.Tag:map[string]any{"managedListener":map[string]any{"enabled":false}}});err!=nil {t.Fatal(err)}
 if len(cfg.InboundConfigs)!=1 || cfg.InboundConfigs[0].Port!=443 {t.Fatal("disabled listener must not change original node")}
}

func TestXHTTPManagedGatewayPreservesMetadataAfterSanitization(t *testing.T) {
    base := mockTLSXHTTP(t)
    // GetXrayConfig rewrites row.StreamSettings without client-only settings;
    // the generator must never try to recover downloadSettings from that row.
    row := &model.Inbound{Id:11,Tag:base.Tag,Enable:true,Protocol:model.VLESS,StreamSettings:string(base.StreamSettings)}
    snapshot := map[string]map[string]any{base.Tag:mockManagedDown()}
    cfg := &xray.Config{InboundConfigs:[]xray.InboundConfig{base}}
    if err:=injectManagedXHTTPGateways(cfg,[]*model.Inbound{row},snapshot);err!=nil {t.Fatal(err)}
    if len(cfg.InboundConfigs)!=3 || cfg.InboundConfigs[2].Port!=9443 {
        t.Fatalf("managed reality gateway silently absent after sanitization: %#v",cfg.InboundConfigs)
    }
}

func TestXHTTPManagedGatewayConfigurableMinVersion(t *testing.T) {
  base:=mockTLSXHTTP(t)
  down:=mockManagedDown()
  settings:=down["managedListener"].(map[string]any)["realitySettings"].(map[string]any)
  settings["minClientVer"]="2.4.6"
  got,err:=xhttpGatewayConfig(base,11,down,map[int]string{443:base.Tag})
  if err!=nil {t.Fatal(err)}
  reality:=xhttpMap(got[2].StreamSettings)["realitySettings"].(map[string]any)
  if reality["minClientVer"]!="2.4.6" {t.Fatalf("minClientVer should be editable: %#v",reality)}
  settings["minClientVer"]="bad-version"
  if _,err=xhttpGatewayConfig(base,11,down,map[int]string{443:base.Tag});err==nil || !strings.Contains(err.Error(),"minClientVer") {
    t.Fatalf("invalid version must be rejected: %v",err)
  }
}
