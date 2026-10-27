package service

// XHTTP paired TLS/REALITY entrypoints are generated only for explicit
// opt-in on a VLESS/XHTTP/TLS inbound. Both outer gateways use RAW VLESS
// fallback to the same internal plaintext XHTTP listener, as in the official
// Xray 5-in-1 layout. Generating two independent XHTTP inbounds is incorrect
// because upload/download sessions would not be shared.
import (
  "encoding/json"
  "fmt"
  "strings"

  "github.com/mhsanaei/3x-ui/v3/internal/database/model"
  "github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
  "github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func marshalXHTTPGateway(v any) (json_util.RawMessage,error) {
  b,err:=json.Marshal(v)
  return json_util.RawMessage(b),err
}

func xhttpMap(b []byte) map[string]any {
  m:=map[string]any{}
  _=json.Unmarshal(b,&m)
  return m
}

func xhttpGatewayConfig(
  base xray.InboundConfig,
  inboundID int,
  down map[string]any,
  used map[int]string,
) ([]xray.InboundConfig,error) {
  orig:=xhttpMap(base.StreamSettings)
  if orig["network"]!="xhttp" || orig["security"]!="tls" || base.Protocol!="vless" {
    return nil,fmt.Errorf("inbound %s: managed REALITY download requires VLESS/XHTTP/TLS",base.Tag)
  }
  managed,_:=down["managedListener"].(map[string]any)
  if managed["enabled"]!=true {return nil,nil}
  if down["security"]!="reality" {return nil,fmt.Errorf("inbound %s: managed listener needs REALITY downlink",base.Tag)}
  port:=0
  switch v:=down["port"].(type) {
  case int: port=v
  case float64: port=int(v)
  }
  if port<1 || port>65535 {return nil,fmt.Errorf("inbound %s: invalid REALITY downlink port",base.Tag)}
  if conflict,exists:=used[port];exists {return nil,fmt.Errorf("inbound %s: REALITY port %d conflicts with %s",base.Tag,port,conflict)}

  reality,_:=managed["realitySettings"].(map[string]any)
  target,_:=reality["target"].(string)
  privateKey,_:=reality["privateKey"].(string)
  publicKey,_:=reality["publicKey"].(string)
  names,ok:=reality["serverNames"].([]any)
  if !ok || len(names)==0 {return nil,fmt.Errorf("inbound %s: REALITY serverNames required",base.Tag)}
  shortIDs,ok:=reality["shortIds"].([]any)
  if !ok || len(shortIDs)==0 {return nil,fmt.Errorf("inbound %s: REALITY shortIds required",base.Tag)}
  clientReality,_:=down["realitySettings"].(map[string]any)
  sni,_:=clientReality["serverName"].(string)
  clientKey,_:=clientReality["publicKey"].(string)
  clientShort,_:=clientReality["shortId"].(string)
  if target=="" || !strings.Contains(target,":") || privateKey=="" || publicKey=="" || clientKey!=publicKey {
    return nil,fmt.Errorf("inbound %s: REALITY target/keypair incomplete or public keys differ",base.Tag)
  }
  haveSNI,haveShort:=false,false
  for _,v:=range names {if v==sni && sni!="" {haveSNI=true}}
  for _,v:=range shortIDs {if v==clientShort {haveShort=true}}
  if !haveSNI || !haveShort {return nil,fmt.Errorf("inbound %s: REALITY client SNI/shortId not allowed by generated gateway",base.Tag)}

  internalName:=fmt.Sprintf("@x3p-xhttp-%d",inboundID)
  fallback:=map[string]any{"decryption":"none","clients":[]any{}, "fallbacks":[]any{map[string]any{"dest":internalName,"xver":0}}}
  fallbackJSON,err:=marshalXHTTPGateway(fallback);if err!=nil{return nil,err}

  // Keep original users, tag, and stats exclusively on the shared internal
  // listener. Only the public entrypoints are synthetic.
  inner:=base
  inner.Listen,_=marshalXHTTPGateway(internalName)
  inner.Port=0
  innerStream:=xhttpMap(base.StreamSettings)
  innerStream["security"]="none"
  delete(innerStream,"tlsSettings")
  delete(innerStream,"realitySettings")
  // XHTTP does not accept a PROXY protocol prefix from these RAW gateways.
  if sockopt,ok:=innerStream["sockopt"].(map[string]any);ok {
    delete(sockopt,"acceptProxyProtocol")
  }
  inner.StreamSettings,err=marshalXHTTPGateway(innerStream)
  if err!=nil{return nil,err}

  front:=base
  front.Tag=base.Tag+"-tls-up"
  front.Settings=fallbackJSON
  frontStream:=map[string]any{"network":"raw","security":"tls","tlsSettings":orig["tlsSettings"]}
  front.StreamSettings,err=marshalXHTTPGateway(frontStream)
  if err!=nil{return nil,err}
  front.Sniffing=nil

  realityFront:=xray.InboundConfig{
    Port:port,
    Protocol:"vless",
    Tag:base.Tag+"-reality-down",
    Settings:fallbackJSON,
  }
  listen,_:=managed["listen"].(string)
  if strings.TrimSpace(listen)=="" {listen="0.0.0.0"}
  realityFront.Listen,err=marshalXHTTPGateway(listen);if err!=nil{return nil,err}
  realityFront.StreamSettings,err=marshalXHTTPGateway(map[string]any{
    "network":"raw",
    "security":"reality",
    "realitySettings":map[string]any{
      "show":false,
      "target":target,
      "xver":0,
      "serverNames":names,
      "privateKey":privateKey,
      "shortIds":shortIDs,
      // Xray 26.7.11+ defaults minClientVer to 26.3.27. Mihomo reports
      // REALITY client version 1.8.2, so the implicit threshold rejects
      // its handshake before XHTTP sessions are reached.
      // Scope this explicit compatibility setting to the opt-in generated
      // REALITY downlink; do not change unrelated inbounds.
      "minClientVer":"1.8.2",
    },
  })
  if err!=nil{return nil,err}
  // Avoid opening an unrelated, potentially vulnerable REALITY listener
  // when the upload and download cannot share the same backend.
  return []xray.InboundConfig{inner,front,realityFront},nil
}

func injectManagedXHTTPGateways(cfg *xray.Config, inbounds []*model.Inbound, downloadsByTag map[string]map[string]any) error {
  used:=map[int]string{}
  for _,entry:=range cfg.InboundConfigs {
    if entry.Port>0 {used[entry.Port]=entry.Tag}
  }
  byTag:=map[string]*model.Inbound{}
  for _,row:=range inbounds {
    if row!=nil && row.Enable && row.NodeID==nil {byTag[row.Tag]=row}
  }
  next:=make([]xray.InboundConfig,0,len(cfg.InboundConfigs)+4)
  for _,entry:=range cfg.InboundConfigs {
    row:=byTag[entry.Tag]
    if row==nil || entry.Protocol!="vless" {
      next=append(next,entry)
      continue
    }
    // XrayService strips client-only metadata from row.StreamSettings before
    // invoking us. Always consume the snapshot captured before sanitization.
    download:=downloadsByTag[entry.Tag]
    managed,_:=download["managedListener"].(map[string]any)
    if managed["enabled"]!=true {
      next=append(next,entry)
      continue
    }
    parts,err:=xhttpGatewayConfig(entry,row.Id,download,used)
    if err!=nil{return fmt.Errorf("XHTTP managed gateway: %w",err)}
    if len(parts)!=3 {return fmt.Errorf("XHTTP managed gateway %s: unexpected shape",entry.Tag)}
    used[parts[2].Port]=parts[2].Tag
    next=append(next,parts...)
  }
  cfg.InboundConfigs=next
  return nil
}
