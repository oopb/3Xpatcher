from pathlib import Path

p = Path('scripts/v20-dynamic-badges-security.py').read_text()
required = [
    "supplementalStream.isReality",
    "supplementalStream.isTls",
    "naiveNetwork === 'tcp'",
    "naiveNetwork === 'udp'",
    "supplementalSettings.transport",
    "record.protocol === 'shadowtls'",
    "ShadowTlsSecurityFields",
    "protocol === Protocols.SHADOWTLS",
]
for token in required:
    if token not in p:
        raise SystemExit(f'missing V20 contract token: {token}')
if '<Tag key=\"tls\" color=\"blue\">TLS</Tag>,' in p.split("record.protocol === 'shadowtls'", 1)[1].split("record.protocol === 'naive'", 1)[0]:
    raise SystemExit('ShadowTLS must not advertise generic TLS security')
print('v20 dynamic badge/security contract: OK')
