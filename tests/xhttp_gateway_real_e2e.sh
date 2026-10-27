#!/usr/bin/env bash
# Real Xray-core TLS/RAW fallback -> shared XHTTP backend smoke test.
set -Eeuo pipefail
: "$XRAY_BIN"
dir="$(mktemp -d)"
PY_PID=""; CLI_PID=""; SRV_PID=""; SPLIT_PID=""
cleanup() {
  for pid in "$PY_PID" "$CLI_PID" "$SRV_PID" "$SPLIT_PID"; do
    if [ -n "$pid" ]; then kill "$pid" 2>/dev/null || true; fi
  done
  rm -rf "$dir"
}
trap cleanup EXIT
openssl req -x509 -newkey rsa:2048 -nodes -keyout "$dir/tls.key" -out "$dir/tls.pem" \
  -days 1 -subj '/CN=cdn.example.com' >/dev/null 2>&1
cert_sha="$(openssl x509 -in "$dir/tls.pem" -noout -fingerprint -sha256 | cut -d= -f2 | tr -d ':')"
keypair="$("$XRAY_BIN" x25519)"
private_key="$(printf '%s\n' "$keypair" | sed -n 's/^PrivateKey: //p')"
public_key="$(printf '%s\n' "$keypair" | sed -n 's/^Password (PublicKey): //p')"
if [ -z "$private_key" ] || [ -z "$public_key" ]; then
  echo 'Failed to generate Xray REALITY test keypair'
  exit 1
fi
printf 'XHTTP_GATEWAY_REAL_E2E_OK\n' > "$dir/index.html"
python3 -m http.server 18080 --bind 127.0.0.1 --directory "$dir" >"$dir/http.log" 2>&1 &
PY_PID=$!
cat >"$dir/server.json" <<EOF
{
  "log": {"loglevel":"debug"},
  "inbounds": [
    {
      "tag": "test-tls-raw",
      "listen":"127.0.0.1","port":19043,"protocol":"vless",
      "settings":{"decryption":"none","clients":[],"fallbacks":[{"dest":"@x3p-e2e","xver":0}]},
      "streamSettings":{"network":"raw","security":"tls","tlsSettings":{"alpn":["h2","http/1.1"],"certificates":[{"certificateFile":"$dir/tls.pem","keyFile":"$dir/tls.key"}]}}
    },
    {
      "tag":"test-reality-raw",
      "listen":"127.0.0.1","port":19443,"protocol":"vless",
      "settings":{"decryption":"none","clients":[],"fallbacks":[{"dest":"@x3p-e2e","xver":0}]},
      "streamSettings":{"network":"raw","security":"reality","realitySettings":{
        "show":false,"target":"www.cloudflare.com:443","xver":0,
        "serverNames":["www.cloudflare.com"],"privateKey":"$private_key","shortIds":["ab12"]
      }}
    },
    {
      "tag": "test-shared-xhttp","listen":"@x3p-e2e","port":0,"protocol":"vless",
      "settings":{"decryption":"none","clients":[{"id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}]},
      "streamSettings":{"network":"xhttp","security":"none","xhttpSettings":{"path":"/test_shared","mode":"auto"}}
    }
  ],
  "outbounds":[{"protocol":"freedom","tag":"direct"}]
}
EOF
cat >"$dir/client.json" <<EOF
{
  "log":{"loglevel":"debug"},
  "inbounds":[{"tag":"socks","listen":"127.0.0.1","port":11080,"protocol":"socks","settings":{"auth":"noauth","udp":false}}],
  "outbounds":[
    {
      "tag":"proxy","protocol":"vless",
      "settings":{"vnext":[{"address":"127.0.0.1","port":19043,"users":[{"id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","encryption":"none"}]}]},
      "streamSettings":{"network":"xhttp","security":"tls","tlsSettings":{"serverName":"cdn.example.com","pinnedPeerCertSha256":"$cert_sha","alpn":["h2"],"fingerprint":"chrome"},"xhttpSettings":{"host":"cdn.example.com","path":"/test_shared","mode":"auto"}}
    }
  ]
}
EOF
"$XRAY_BIN" run -test -c "$dir/server.json" >"$dir/validate.log" 2>&1 || { cat "$dir/validate.log"; exit 1; }
"$XRAY_BIN" run -c "$dir/server.json" >"$dir/server.log" 2>&1 &
SRV_PID=$!
"$XRAY_BIN" run -c "$dir/client.json" >"$dir/client.log" 2>&1 &
CLI_PID=$!
sleep 3
# Xray 26.x deliberately blocks loopback targets through Freedom to prevent
# SSRF, so test with an external HTTPS destination instead of 127.0.0.1:18080.
if ! curl -fsS --max-time 15 --retry 1 --retry-delay 1 \
  --socks5-hostname 127.0.0.1:11080 \
  https://example.com/ | grep -iq 'example'; then
  echo "::error::TLS/RAW gateway to XHTTP shared backend FAILED"
  echo "======= server log ======="
  tail -100 "$dir/server.log"
  echo "======= client log ======="
  tail -100 "$dir/client.log"
  exit 1
fi
echo 'PASS: TLS/RAW gateway -> abstract UDS XHTTP backend -> VLESS user -> HTTP target.'
# A second client uses EXACTLY the same XHTTP backend/UUID/path, with TLS
# uplink but independent REALITY download over port 19443.
cat >"$dir/split-client.json" <<EOF
{
  "log":{"loglevel":"debug"},
  "inbounds":[{"tag":"socks","listen":"127.0.0.1","port":11081,"protocol":"socks","settings":{"auth":"noauth","udp":false}}],
  "outbounds":[
    {"tag":"proxy","protocol":"vless",
      "settings":{"vnext":[{"address":"127.0.0.1","port":19043,"users":[{"id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","encryption":"none"}]}]},
      "streamSettings":{"network":"xhttp","security":"tls",
        "tlsSettings":{"serverName":"cdn.example.com","pinnedPeerCertSha256":"$cert_sha","alpn":["h2"],"fingerprint":"chrome"},
        "xhttpSettings":{"host":"cdn.example.com","path":"/test_shared","mode":"auto",
          "extra":{"downloadSettings":{
            "address":"127.0.0.1","port":19443,"network":"xhttp","security":"reality",
            "realitySettings":{"serverName":"www.cloudflare.com","fingerprint":"chrome","publicKey":"$public_key","shortId":"ab12"},
            "xhttpSettings":{"host":"","path":"/test_shared","mode":"auto"}
          }}
        }
      }
    }
  ]
}
EOF
"$XRAY_BIN" run -test -c "$dir/split-client.json" >"$dir/validate-split.log" 2>&1 || {
  cat "$dir/validate-split.log"; exit 1;
}
"$XRAY_BIN" run -c "$dir/split-client.json" >"$dir/split-client.log" 2>&1 &
SPLIT_PID=$!
sleep 2
if ! curl -fsS --max-time 15 --socks5-hostname 127.0.0.1:11081   https://example.com/ | grep -iq 'example'; then
  echo "::error::REALITY downlink split to XHTTP backend FAILED"
  echo "======= server ======="
  tail -120 "$dir/server.log"
  echo "======= split client ======="
  tail -120 "$dir/split-client.log"
  exit 1
fi
echo 'PASS: TLS upload + REALITY download -> same inner XHTTP backend and user.'

