#!/usr/bin/env python3
import sys
from v2_patchlib import Patcher

rep = Patcher(sys.argv[1]).rep

# Heal pre-existing SS2022 rows during Xray config construction as well as at
# save time. This matters after upgrading from a build where browser autofill
# was able to persist an invalid server/client PSK: the panel must recover
# before the operator opens and re-saves the inbound, and subscriptions must
# see the same repaired DB value.
rep(
    'internal/web/service/xray.go',
    '"github.com/mhsanaei/3x-ui/v3/internal/database/model"',
    '"github.com/mhsanaei/3x-ui/v3/internal/database"\n\t"github.com/mhsanaei/3x-ui/v3/internal/database/model"',
)
rep(
    'internal/web/service/xray.go',
    '''\t\tif inbound.Protocol == model.Shadowsocks {\n\t\t\tif healed, ok := model.HealShadowsocksClientMethods(inbound.Settings); ok {\n\t\t\t\tinbound.Settings = healed\n\t\t\t}\n\t\t}''',
    '''\t\tif inbound.Protocol == model.Shadowsocks {\n\t\t\tsettingsChanged := false\n\t\t\tif healed, ok := model.HealShadowsocksClientMethods(inbound.Settings); ok {\n\t\t\t\tinbound.Settings = healed\n\t\t\t\tsettingsChanged = true\n\t\t\t}\n\t\t\tif healed, ok := normalizeShadowsocks2022Keys(inbound.Settings); ok {\n\t\t\t\tinbound.Settings = healed\n\t\t\t\tsettingsChanged = true\n\t\t\t\tlogger.Warningf("Inbound %q: regenerated invalid Shadowsocks-2022 PSK(s); clients must refresh their subscription", inbound.Tag)\n\t\t\t}\n\t\t\tif settingsChanged && inbound.Id > 0 {\n\t\t\t\tif err := database.GetDB().Model(&model.Inbound{}).Where("id = ?", inbound.Id).Update("settings", inbound.Settings).Error; err != nil {\n\t\t\t\t\tlogger.Warningf("Inbound %q: failed to persist healed Shadowsocks settings: %v", inbound.Tag, err)\n\t\t\t\t}\n\t\t\t}\n\t\t}''',
)

# Defense in depth for Mihomo/Clash: even a direct DB edit or a partially
# restored backup must not make one malformed SS2022 entry invalidate the whole
# subscription. Invalid nodes are omitted until the DB heal/save path repairs
# them. Standard Base64 is required by the 2022 key specification.
rep(
    'internal/sub/clash_service.go',
    '''import (\n\t"errors"''',
    '''import (\n\t"encoding/base64"\n\t"errors"''',
)
rep(
    'internal/sub/clash_service.go',
    '''type SubClashService struct {''',
    '''func validSS2022Key(method, key string) bool {\n\tif !strings.HasPrefix(method, "2022-blake3-") {\n\t\treturn key != ""\n\t}\n\texpected := 32\n\tif method == "2022-blake3-aes-128-gcm" {\n\t\texpected = 16\n\t}\n\tdecoded, err := base64.StdEncoding.DecodeString(key)\n\treturn err == nil && len(decoded) == expected\n}\n\ntype SubClashService struct {''',
)
rep(
    'internal/sub/clash_service.go',
    '''\t\tproxy["cipher"] = method\n\t\tif strings.HasPrefix(method, "2022") {\n\t\t\tif serverPassword, ok := inboundSettings["password"].(string); ok && serverPassword != "" {\n\t\t\t\tproxy["password"] = fmt.Sprintf("%s:%s", serverPassword, client.Password)\n\t\t\t}\n\t\t}''',
    '''\t\tproxy["cipher"] = method\n\t\tif strings.HasPrefix(method, "2022") {\n\t\t\tif !validSS2022Key(method, client.Password) {\n\t\t\t\treturn nil\n\t\t\t}\n\t\t\tserverPassword, ok := inboundSettings["password"].(string)\n\t\t\tif !ok || !validSS2022Key(method, serverPassword) {\n\t\t\t\treturn nil\n\t\t\t}\n\t\t\tproxy["password"] = fmt.Sprintf("%s:%s", serverPassword, client.Password)\n\t\t}''',
)
