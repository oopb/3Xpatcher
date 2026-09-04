#!/usr/bin/env python3
import sys
from v2_patchlib import Patcher

rep = Patcher(sys.argv[1]).rep

# ---------------------------------------------------------------------------
# Supplemental online state must not live exclusively on xray.Process. A panel
# may legitimately run only sing-box/Mieru inbounds, in which case the native
# getters used by /client/onlines and the dashboard used to return an empty set.
# Merge the service-level supplemental cache with Xray/local-node state.
# ---------------------------------------------------------------------------
rep(
    'internal/web/service/inbound_node.go',
    '''func (s *InboundService) GetOnlineClients() []string {\n\tprocess := currentXrayProcess()\n\tif process == nil {\n\t\treturn []string{}\n\t}\n\treturn process.GetOnlineClients()\n}''',
    '''func (s *InboundService) GetOnlineClients() []string {\n\tsupplemental, _ := supplementalOnlineSnapshot()\n\tprocess := currentXrayProcess()\n\tif process == nil {\n\t\treturn supplemental\n\t}\n\treturn mergeEmails(process.GetOnlineClients(), supplemental)\n}''',
)

rep(
    'internal/web/service/inbound_node.go',
    '''func (s *InboundService) GetOnlineClientsByGuid() map[string][]string {\n\tprocess := currentXrayProcess()\n\tif process == nil {\n\t\treturn map[string][]string{}\n\t}\n\tout := process.GetMergedNodeTrees()\n\tif local := process.GetLocalOnlineClients(); len(local) > 0 {\n\t\tif guid := s.panelGuid(); guid != "" {\n\t\t\tout[guid] = mergeEmails(out[guid], local)\n\t\t}\n\t}\n\treturn out\n}''',
    '''func (s *InboundService) GetOnlineClientsByGuid() map[string][]string {\n\tout := map[string][]string{}\n\tif process := currentXrayProcess(); process != nil {\n\t\tout = process.GetMergedNodeTrees()\n\t\tif out == nil {\n\t\t\tout = map[string][]string{}\n\t\t}\n\t\tif local := process.GetLocalOnlineClients(); len(local) > 0 {\n\t\t\tif guid := s.panelGuid(); guid != "" {\n\t\t\t\tout[guid] = mergeEmails(out[guid], local)\n\t\t\t}\n\t\t}\n\t}\n\tif supplemental, _ := supplementalOnlineSnapshot(); len(supplemental) > 0 {\n\t\tif guid := s.panelGuid(); guid != "" {\n\t\t\tout[guid] = mergeEmails(out[guid], supplemental)\n\t\t}\n\t}\n\treturn out\n}''',
)

rep(
    'internal/web/service/inbound_node.go',
    '''func (s *InboundService) GetActiveInboundsByGuid() map[string][]string {\n\tprocess := currentXrayProcess()\n\tif process == nil {\n\t\treturn map[string][]string{}\n\t}\n\tout := process.GetMergedActiveInboundTrees()\n\tactive := process.GetLocalActiveInbounds()\n\tif len(active) == 0 {\n\t\treturn out\n\t}\n\tguid := s.panelGuid()\n\tif guid == "" {\n\t\treturn out\n\t}\n\tout[guid] = mergeEmails(out[guid], active)\n\treturn out\n}''',
    '''func (s *InboundService) GetActiveInboundsByGuid() map[string][]string {\n\tout := map[string][]string{}\n\tif process := currentXrayProcess(); process != nil {\n\t\tout = process.GetMergedActiveInboundTrees()\n\t\tif out == nil {\n\t\t\tout = map[string][]string{}\n\t\t}\n\t\tif active := process.GetLocalActiveInbounds(); len(active) > 0 {\n\t\t\tif guid := s.panelGuid(); guid != "" {\n\t\t\t\tout[guid] = mergeEmails(out[guid], active)\n\t\t\t}\n\t\t}\n\t}\n\tif _, supplemental := supplementalOnlineSnapshot(); len(supplemental) > 0 {\n\t\tif guid := s.panelGuid(); guid != "" {\n\t\t\tout[guid] = mergeEmails(out[guid], supplemental)\n\t\t}\n\t}\n\treturn out\n}''',
)

# Dashboard/system-metric online count should use the merged native service
# view too, otherwise a supplemental-only panel still shows zero online users.
rep(
    'internal/web/service/server.go',
    '''\tonline := 0\n\tif process := currentXrayProcess(); process != nil && process.IsRunning() {\n\t\tonline = len(process.GetOnlineClients())\n\t}\n\tsystemMetrics.append("online", t, float64(online))''',
    '''\tonline := len((&InboundService{}).GetOnlineClients())\n\tsystemMetrics.append("online", t, float64(online))''',
)

# ---------------------------------------------------------------------------
# ShadowTLS inner Shadowsocks-2022 key: mirror native 3x-ui's Shadowsocks logic.
# The custom form used to rely on a manual Generate button only. Heal empty or
# invalid keys automatically on load/method change, and keep Generate as an
# explicit rotate action with form-state updates.
# ---------------------------------------------------------------------------
rep(
    'frontend/src/pages/inbounds/form/protocols/singbox.tsx',
    '''import { Alert, Button, Divider, Input, InputNumber, Select, Space, Switch } from 'antd';\nimport { useFormContext, useWatch } from 'react-hook-form';''',
    '''import { useEffect } from 'react';\n\nimport { Alert, Button, Divider, Input, InputNumber, Select, Space, Switch } from 'antd';\nimport { useFormContext, useWatch } from 'react-hook-form';''',
)
rep(
    'frontend/src/pages/inbounds/form/protocols/singbox.tsx',
    '''  const innerMethod = useWatch({ control, name: 'settings.innerMethod' }) as string | undefined;\n  const regenerateInnerPassword = () =>\n    setValue(\n      'settings.innerPassword',\n      RandomUtil.randomShadowsocksPassword(\n        innerMethod === '2022-blake3-aes-256-gcm' || innerMethod === '2022-blake3-chacha20-poly1305'\n          ? innerMethod\n          : '2022-blake3-aes-128-gcm',\n      ),\n      { shouldDirty: true },\n    );''',
    '''  const innerMethod = useWatch({ control, name: 'settings.innerMethod' }) as string | undefined;\n  const innerPassword = (useWatch({ control, name: 'settings.innerPassword' }) as string | undefined) || '';\n  const keyMethod =\n    innerMethod === '2022-blake3-aes-256-gcm' || innerMethod === '2022-blake3-chacha20-poly1305'\n      ? innerMethod\n      : '2022-blake3-aes-128-gcm';\n\n  const regenerateInnerPassword = () =>\n    setValue('settings.innerPassword', RandomUtil.randomShadowsocksPassword(keyMethod), {\n      shouldDirty: true,\n      shouldTouch: true,\n      shouldValidate: true,\n    });\n\n  useEffect(() => {\n    if (RandomUtil.isShadowsocks2022Password(innerPassword, keyMethod)) return;\n    setValue('settings.innerPassword', RandomUtil.randomShadowsocksPassword(keyMethod), {\n      shouldDirty: true,\n      shouldTouch: true,\n      shouldValidate: true,\n    });\n  }, [innerPassword, keyMethod, setValue]);''',
)
rep(
    'frontend/src/pages/inbounds/form/protocols/singbox.tsx',
    '''        <Space.Compact block><Input.Password /><Button onClick={regenerateInnerPassword}>Generate</Button></Space.Compact>''',
    '''        <Space.Compact block>\n          <Input.Password autoComplete="new-password" data-lpignore="true" data-1p-ignore="true" />\n          <Button htmlType="button" onClick={regenerateInnerPassword}>Generate</Button>\n        </Space.Compact>''',
)
