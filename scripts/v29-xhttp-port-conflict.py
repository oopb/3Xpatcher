#!/usr/bin/env python3
"""Extend v3.11 upstream transactional checks to managed XHTTP REALITY TCP ports."""
from pathlib import Path
import sys
p = Path(sys.argv[1]) / "internal/web/service/port_conflict.go"
if not p.exists():
    print("No upstream port_conflict.go; legacy compatibility unchanged")
    raise SystemExit(0)
s = p.read_text()
a = """\t}
\treturn nil, nil
}

// amneziawgForwardedPortOwner"""
b = """\t}
\tif conflict, err := checkManagedXHTTPPortConflictTx(db, inbound, ignoreId); err != nil || conflict != nil {
\t\treturn conflict, err
\t}
\treturn nil, nil
}

// amneziawgForwardedPortOwner"""
if s.count(a) != 1:
    raise SystemExit("port_conflict.go anchor missing or ambiguous")
s = s.replace(a,b,1)
s += r'''

// An extra TCP socket created by a managed XHTTP REALITY download is not
// represented by inbound.Port, so upstream's standard check cannot see it.
func managedXHTTPClaim(row *model.Inbound) (int, bindAddr, bool, error) {
    if row == nil || row.NodeID != nil || row.Protocol != model.VLESS {
        return 0, bindAddr{}, false, nil
    }
    var ss struct {
        Network string @@json:"network"@@
        Security string @@json:"security"@@
        XHTTP struct {
            Download struct {
                Port int @@json:"port"@@
                Security string @@json:"security"@@
                Managed struct {
                    Enabled bool @@json:"enabled"@@
                    Listen string @@json:"listen"@@
                } @@json:"managedListener"@@
            } @@json:"downloadSettings"@@
        } @@json:"xhttpSettings"@@
    }
    if json.Unmarshal([]byte(row.StreamSettings), &ss) != nil {
        return 0, bindAddr{}, false, nil
    }
    d := ss.XHTTP.Download
    if !d.Managed.Enabled { return 0, bindAddr{}, false, nil }
    if ss.Network != "xhttp" || ss.Security != "tls" || d.Security != "reality" {
        return 0, bindAddr{}, false, fmt.Errorf("managed REALITY download requires VLESS/XHTTP/TLS uplink and REALITY downlink")
    }
    if d.Port < 1 || d.Port > 65535 {
        return 0, bindAddr{}, false, fmt.Errorf("invalid managed REALITY TCP download port %d", d.Port)
    }
    listen := strings.TrimSpace(d.Managed.Listen)
    if listen == "" { listen = "0.0.0.0" }
    if net.ParseIP(listen) == nil {
        return 0, bindAddr{}, false, fmt.Errorf("invalid REALITY download listen address %q", listen)
    }
    return d.Port, bindAddr{listen:listen}, true, nil
}
func managedConflict(row *model.Inbound, port int, addr bindAddr, suffix string) *portConflictDetail {
    return &portConflictDetail{InboundID: row.Id, Remark: row.Remark+suffix,
        Tag: row.Tag, Listen: addr.listen, Port: port, Transports: transportTCP}
}
func checkManagedXHTTPPortConflictTx(db *gorm.DB, candidate *model.Inbound, ignoreId int) (*portConflictDetail, error) {
    if candidate == nil || candidate.NodeID != nil { return nil, nil }
    port, addr, enabled, err := managedXHTTPClaim(candidate)
    if err != nil { return nil, err }
    tcp := inboundTransports(candidate.Protocol,candidate.StreamSettings,candidate.Settings)&transportTCP != 0
    if enabled {
        if tcp && port == candidate.Port && listenOverlaps(addr,inboundBindAddr(candidate)) {
            return managedConflict(candidate,port,inboundBindAddr(candidate)," (TLS uplink)"),nil
        }
        if port == reservedAPIPort() && listenOverlaps(addr,loopbackBind) {
            return &portConflictDetail{Tag:"api",Listen:"127.0.0.1",Port:port,Transports:transportTCP},nil
        }
    }
    var rows []*model.Inbound
    q := db.Model(model.Inbound{}).Where("node_id IS NULL")
    if ignoreId > 0 { q=q.Where("id != ?",ignoreId) }
    if err:=q.Find(&rows).Error;err!=nil{return nil,err}
    for _,other:=range rows {
        op,oa,oe,err:=managedXHTTPClaim(other)
        if err!=nil{return nil,fmt.Errorf("existing inbound #%d: %w",other.Id,err)}
        if tcp && oe && candidate.Port==op && listenOverlaps(inboundBindAddr(candidate),oa) {
            return managedConflict(other,op,oa," (REALITY downlink)"),nil
        }
        if !enabled {continue}
        if port==other.Port && inboundTransports(other.Protocol,other.StreamSettings,other.Settings)&transportTCP!=0 &&
            listenOverlaps(addr,inboundBindAddr(other)) {
            return managedConflict(other,other.Port,inboundBindAddr(other),""),nil
        }
        if oe && port==op && listenOverlaps(addr,oa) {
            return managedConflict(other,op,oa," (REALITY downlink)"),nil
        }
    }
    if !enabled {return nil,nil}
    // Editing an unchanged already-running managed port is not an OS
    // conflict with an unrelated process; all other new binds are probed.
    owned:=false
    if ignoreId>0 {
        var previous model.Inbound
        res:=db.Where("id = ?",ignoreId).Find(&previous)
        if res.Error!=nil{return nil,res.Error}
        if res.RowsAffected>0 && previous.Enable {
            oldPort,oldAddr,oldOn,err:=managedXHTTPClaim(&previous)
            if err!=nil{return nil,err}
            owned=oldOn && oldPort==port && listenOverlaps(oldAddr,addr)
        }
    }
    if !owned {
        dest:=net.JoinHostPort(addr.listen,fmt.Sprintf("%d",port))
        listener,err:=net.Listen("tcp",dest)
        if err!=nil{return nil,fmt.Errorf("managed REALITY TCP port %d on %s unavailable: %w",port,addr.listen,err)}
        _=listener.Close()
    }
    return nil,nil
}
'''.replace("@@",chr(96))
p.write_text(s)
print("Added managed XHTTP port checks to upstream create/edit/enable transaction")
