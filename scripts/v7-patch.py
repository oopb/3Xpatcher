#!/usr/bin/env python3
import sys
from v2_patchlib import Patcher

rep = Patcher(sys.argv[1]).rep

# ---------------------------------------------------------------------------
# sing-box: enable the stats API in the rendered config. 3Xpatcher ships a
# source-pinned sing-box build with with_v2ray_api, and the service is bound to
# loopback only. The API uses the same stats wire format as Xray/V2Ray, so the
# panel can fold counters into its native traffic database.
# ---------------------------------------------------------------------------
rep(
    'internal/singbox/config.go',
    '''func BuildConfig(records []InboundRecord) ([]byte, error) {\n\tinbounds := make([]any, 0, len(records)+2)\n\tseenTags := make(map[string]struct{})''',
    '''func BuildConfig(records []InboundRecord) ([]byte, error) {\n\tinbounds := make([]any, 0, len(records)+2)\n\tseenTags := make(map[string]struct{})\n\tstatsInbounds := make([]string, 0, len(records))\n\tstatsUsers := make([]string, 0)\n\tstatsUserSet := make(map[string]struct{})''',
)
rep(
    'internal/singbox/config.go',
    '''\t\tseenTags[r.Tag] = struct{}{}\n\t\trendered, extraTags, err := renderInbound(r)''',
    '''\t\tseenTags[r.Tag] = struct{}{}\n\t\tstatsInbounds = append(statsInbounds, r.Tag)\n\t\tfor _, user := range statsUsersForRecord(r) {\n\t\t\tif user == "" {\n\t\t\t\tcontinue\n\t\t\t}\n\t\t\tif _, exists := statsUserSet[user]; exists {\n\t\t\t\tcontinue\n\t\t\t}\n\t\t\tstatsUserSet[user] = struct{}{}\n\t\t\tstatsUsers = append(statsUsers, user)\n\t\t}\n\t\trendered, extraTags, err := renderInbound(r)''',
)
rep(
    'internal/singbox/config.go',
    '''\treturn json.MarshalIndent(map[string]any{"log": map[string]any{"level": "info", "timestamp": true}, "inbounds": inbounds}, "", "  ")\n}\n\nfunc validateCommon''',
    '''\troot := map[string]any{\n\t\t"log":      map[string]any{"level": "info", "timestamp": true},\n\t\t"inbounds": inbounds,\n\t\t"experimental": map[string]any{\n\t\t\t"v2ray_api": map[string]any{\n\t\t\t\t"listen": StatsListenAddress,\n\t\t\t\t"stats": map[string]any{\n\t\t\t\t\t"enabled":  true,\n\t\t\t\t\t"inbounds": statsInbounds,\n\t\t\t\t\t"users":    statsUsers,\n\t\t\t\t},\n\t\t\t},\n\t\t},\n\t}\n\treturn json.MarshalIndent(root, "", "  ")\n}\n\nfunc statsUsersForRecord(r InboundRecord) []string {\n\tswitch r.Protocol {\n\tcase ProtocolTUIC:\n\t\tvar s TUICSettings\n\t\tif json.Unmarshal(r.Settings, &s) != nil {\n\t\t\treturn nil\n\t\t}\n\t\tout := make([]string, 0, len(s.Users))\n\t\tfor _, u := range s.Users {\n\t\t\tout = append(out, u.Name)\n\t\t}\n\t\treturn out\n\tcase ProtocolAnyTLS:\n\t\tvar s AnyTLSSettings\n\t\tif json.Unmarshal(r.Settings, &s) != nil {\n\t\t\treturn nil\n\t\t}\n\t\tout := make([]string, 0, len(s.Users))\n\t\tfor _, u := range s.Users {\n\t\t\tout = append(out, u.Name)\n\t\t}\n\t\treturn out\n\tcase ProtocolShadowTLS:\n\t\tvar s ShadowTLSSettings\n\t\tif json.Unmarshal(r.Settings, &s) != nil {\n\t\t\treturn nil\n\t\t}\n\t\tout := make([]string, 0, len(s.Users))\n\t\tfor _, u := range s.Users {\n\t\t\tout = append(out, u.Name)\n\t\t}\n\t\treturn out\n\tcase ProtocolNaive:\n\t\tvar s NaiveSettings\n\t\tif json.Unmarshal(r.Settings, &s) != nil {\n\t\t\treturn nil\n\t\t}\n\t\tout := make([]string, 0, len(s.Users))\n\t\tfor _, u := range s.Users {\n\t\t\tout = append(out, u.Username)\n\t\t}\n\t\treturn out\n\tdefault:\n\t\treturn nil\n\t}\n}\n\nfunc validateCommon''',
)

# ---------------------------------------------------------------------------
# Shadowsocks 2022: use the same key-size rules already present in 3x-ui for
# client PSKs, but apply them to the inbound/server PSK too. This makes browser
# autofill harmless: an invalid text password is replaced before persistence.
# Existing client-key normalization remains in place and both run together.
# ---------------------------------------------------------------------------
rep(
    'internal/web/service/client_crud.go',
    '''func applyShadowsocksClientMethod(clients []any, settings map[string]any) {''',
    '''func normalizeShadowsocksServerKey(settings string) (string, bool) {\n\tmethod := shadowsocksMethodFromSettings(settings)\n\tif shadowsocksKeyBytes(method) == 0 {\n\t\treturn settings, false\n\t}\n\tvar m map[string]any\n\tif err := json.Unmarshal([]byte(settings), &m); err != nil {\n\t\treturn settings, false\n\t}\n\tif password, _ := m["password"].(string); validShadowsocksClientKey(method, password) {\n\t\treturn settings, false\n\t}\n\tm["password"] = randomShadowsocksClientKey(method)\n\tbs, err := json.MarshalIndent(m, "", "  ")\n\tif err != nil {\n\t\treturn settings, false\n\t}\n\treturn string(bs), true\n}\n\nfunc normalizeShadowsocks2022Keys(settings string) (string, bool) {\n\tserverNormalized, serverChanged := normalizeShadowsocksServerKey(settings)\n\tclientNormalized, clientChanged := normalizeShadowsocksClientKeys(serverNormalized)\n\treturn clientNormalized, serverChanged || clientChanged\n}\n\nfunc applyShadowsocksClientMethod(clients []any, settings map[string]any) {''',
)
rep(
    'internal/web/service/inbound.go',
    'normalizeShadowsocksClientKeys(inbound.Settings)',
    'normalizeShadowsocks2022Keys(inbound.Settings)',
    count=2,
)

# ---------------------------------------------------------------------------
# Shadowsocks UI: auto-generate a valid PSK whenever a 2022 cipher sees an
# empty/invalid value; mark generated values dirty/validated and discourage
# password managers from replacing protocol keys. Generate remains available
# as an explicit rotate action.
# ---------------------------------------------------------------------------
rep(
    'frontend/src/pages/inbounds/form/protocols/shadowsocks.tsx',
    "import { useTranslation } from 'react-i18next';",
    "import { useEffect } from 'react';\nimport { useTranslation } from 'react-i18next';",
)
rep(
    'frontend/src/pages/inbounds/form/protocols/shadowsocks.tsx',
    "import { useFormContext } from 'react-hook-form';",
    "import { useFormContext, useWatch } from 'react-hook-form';",
)
rep(
    'frontend/src/pages/inbounds/form/protocols/shadowsocks.tsx',
    '''  const { getValues, setValue } = useFormContext();\n  return (''',
    '''  const { control, getValues, setValue } = useFormContext();\n  const method = (useWatch({ control, name: 'settings.method' }) as string | undefined) || '';\n  const password = (useWatch({ control, name: 'settings.password' }) as string | undefined) || '';\n\n  useEffect(() => {\n    if (!method.startsWith('2022') || RandomUtil.isShadowsocks2022Password(password, method)) return;\n    setValue('settings.password', RandomUtil.randomShadowsocksPassword(method), {\n      shouldDirty: true,\n      shouldTouch: true,\n      shouldValidate: true,\n    });\n  }, [method, password, setValue]);\n\n  return (''',
)
rep(
    'frontend/src/pages/inbounds/form/protocols/shadowsocks.tsx',
    '''          setValue('settings.password', RandomUtil.randomShadowsocksPassword(v as string));''',
    '''          setValue('settings.password', RandomUtil.randomShadowsocksPassword(v as string), { shouldDirty: true, shouldTouch: true, shouldValidate: true });''',
)
rep(
    'frontend/src/pages/inbounds/form/protocols/shadowsocks.tsx',
    '''              <Input style={{ width: 'calc(100% - 32px)' }} />''',
    '''              <Input autoComplete="new-password" data-lpignore="true" data-1p-ignore="true" style={{ width: 'calc(100% - 32px)' }} />''',
)
rep(
    'frontend/src/pages/inbounds/form/protocols/shadowsocks.tsx',
    '''            <Button\n              aria-label={t('regenerate')}''',
    '''            <Button\n              htmlType="button"\n              aria-label={t('regenerate')}''',
)
rep(
    'frontend/src/pages/inbounds/form/protocols/shadowsocks.tsx',
    '''                setValue(\n                  'settings.password',\n                  RandomUtil.randomShadowsocksPassword(method as string),\n                );''',
    '''                setValue(\n                  'settings.password',\n                  RandomUtil.randomShadowsocksPassword(method as string),\n                  { shouldDirty: true, shouldTouch: true, shouldValidate: true },\n                );''',
)

rep(
    'frontend/src/pages/clients/ClientFormModal.tsx',
    '''  function regeneratePassword() {\n    methods.setValue(\n      'password',\n      ss2022Method\n        ? RandomUtil.randomShadowsocksPassword(ss2022Method)\n        : RandomUtil.randomLowerAndNum(16),\n    );\n  }''',
    '''  function regeneratePassword() {\n    const nextPassword = ss2022Method\n      ? RandomUtil.randomShadowsocksPassword(ss2022Method)\n      : RandomUtil.randomLowerAndNum(16);\n    methods.setValue('password', nextPassword, {\n      shouldDirty: true,\n      shouldTouch: true,\n      shouldValidate: true,\n    });\n  }''',
)
rep(
    'frontend/src/pages/clients/ClientFormModal.tsx',
    '''      methods.setValue('password', RandomUtil.randomShadowsocksPassword(ss2022Method));''',
    '''      methods.setValue('password', RandomUtil.randomShadowsocksPassword(ss2022Method), { shouldDirty: true, shouldTouch: true, shouldValidate: true });''',
)
rep(
    'frontend/src/pages/clients/ClientFormModal.tsx',
    '''                          <Input\n                            value={password}\n                            style={{ flex: 1 }}''',
    '''                          <Input\n                            value={password}\n                            autoComplete="new-password"\n                            data-lpignore="true"\n                            data-1p-ignore="true"\n                            style={{ flex: 1 }}''',
)
rep(
    'frontend/src/pages/clients/ClientFormModal.tsx',
    '''                          <Button\n                            aria-label={t('regenerate')}\n                            icon={<ReloadOutlined />}\n                            onClick={regeneratePassword}''',
    '''                          <Button\n                            htmlType="button"\n                            aria-label={t('regenerate')}\n                            icon={<ReloadOutlined />}\n                            onClick={regeneratePassword}''',
)

# ---------------------------------------------------------------------------
# Schedule the supplemental collector independently of the Xray collector so
# sing-box/Mieru continue accounting even when Xray has no active inbounds.
# ---------------------------------------------------------------------------
rep(
    'internal/web/web.go',
    '''\tcadenceXrayTraffic   = "@every 5s"\n\tcadenceMtproto       = "@every 10s"''',
    '''\tcadenceXrayTraffic         = "@every 5s"\n\tcadenceSupplementalTraffic = "@every 5s"\n\tcadenceMtproto             = "@every 10s"''',
)
rep(
    'internal/web/web.go',
    '''\tgo func() {\n\t\ttime.Sleep(time.Second * 5)\n\t\t_, _ = s.cron.AddJob(cadenceXrayTraffic, job.NewXrayTrafficJob())\n\t}()\n\n\t// Reconcile mtproto''',
    '''\tgo func() {\n\t\ttime.Sleep(time.Second * 5)\n\t\t_, _ = s.cron.AddJob(cadenceXrayTraffic, job.NewXrayTrafficJob())\n\t}()\n\n\tsupplementalTrafficJob := job.NewSupplementalTrafficJob()\n\t_, _ = s.cron.AddJob(cadenceSupplementalTraffic, supplementalTrafficJob)\n\tgo supplementalTrafficJob.Run()\n\n\t// Reconcile mtproto''',
)
