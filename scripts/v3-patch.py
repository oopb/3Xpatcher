#!/usr/bin/env python3
import sys
from v2_patchlib import Patcher

rep = Patcher(sys.argv[1]).rep

# Supplemental protocols are native multi-user inbounds: expose all of the
# existing 3x-ui attach/detach UI and API paths instead of inventing another
# client manager.
rep(
    'frontend/src/pages/inbounds/list/helpers.ts',
    """    case 'amneziawg':\n      return true;""",
    """    case 'amneziawg':\n    case 'tuic':\n    case 'anytls':\n    case 'shadowtls':\n    case 'naive':\n      return true;""",
)

# Make the existing attach/detach actions directly visible on every multi-user
# inbound row. The same actions remain in the More menu for compatibility.
rep(
    'frontend/src/pages/inbounds/list/RowActions.tsx',
    """      <Dropdown\n        trigger={['click']}""",
    """      {isInboundMultiUser(record) && (\n        <Button\n          type=\"text\"\n          size=\"small\"\n          style={{ fontSize: 16 }}\n          icon={<UsergroupAddOutlined />}\n          aria-label={t('pages.inbounds.attachExistingClients')}\n          onClick={() => onClick('attachExisting')}\n        />\n      )}\n      {isInboundMultiUser(record) && hasClients && (\n        <Button\n          type=\"text\"\n          size=\"small\"\n          style={{ fontSize: 16 }}\n          icon={<UsergroupDeleteOutlined />}\n          aria-label={t('pages.inbounds.detachClients')}\n          onClick={() => onClick('detachClients')}\n        />\n      )}\n      <Dropdown\n        trigger={['click']}""",
)

# For self-signed SNI mode, bypass the native certificate material while still
# reusing 3x-ui's TLS ALPN/version/cipher/curve editor. Normal mode continues
# to use the existing imported/path certificate behavior.
rep(
    'internal/singbox/integrated.go',
    """\ttlsIn, _ := stream[\"tlsSettings\"].(map[string]any)\n\tif tlsIn == nil {\n\t\treturn errors.New(\"TLS settings are missing\")\n\t}\n\ttlsOut := map[string]any{\"enabled\": true}""",
    """\ttlsIn, _ := stream[\"tlsSettings\"].(map[string]any)\n\tif tlsIn == nil {\n\t\treturn errors.New(\"TLS settings are missing\")\n\t}\n\tif mode, _ := settings[\"tlsMode\"].(string); mode == \"self_signed_sni\" {\n\t\treturn installSelfSignedTLS(settings, tlsIn)\n\t}\n\ttlsOut := map[string]any{\"enabled\": true}\n\tcopyNativeTLSOptions(tlsIn, tlsOut)""",
)
