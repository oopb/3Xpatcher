#!/usr/bin/env python3
"""Regression tests for the 3Xpatcher sponsor removal overlay."""
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / "scripts/disable-sponsors.py"


def write(root: Path, rel: str, content: str) -> None:
    path = root / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")


def read(root: Path, rel: str) -> str:
    return (root / rel).read_text(encoding="utf-8")


def fixture(root: Path, layout: str) -> None:
    write(root, "internal/web/service/panel/sponsor.go", "package panel\n")
    write(root, "frontend/src/components/sponsor/SponsorSlot.tsx", "export default function SponsorSlot() { return null; }\n")
    write(
        root,
        "internal/web/controller/index.go",
        'package controller\n'
        'import (\n\t"net/http"\n'
        '\t"github.com/mhsanaei/3x-ui/v3/internal/web/entity"\n)\n'
        'func (a *IndexController) sponsors(c *gin.Context) {\n'
        '\tlist, err := a.panelService.GetSponsors()\n'
        '\tif err != nil {\n'
        '\t\tlogger.Debug("sponsors fetch failed:", err)\n'
        '\t\tc.JSON(http.StatusOK, entity.Msg{Success: false})\n'
        '\t\treturn\n\t}\n'
        '\tjsonObj(c, list, nil)\n}\n'
        'func (a *IndexController) sponsorLogo(c *gin.Context) {\n'
        '\tdata, contentType, err := a.panelService.GetSponsorLogo(c.Param("name"))\n'
        '\tif err != nil {\n'
        '\t\tlogger.Debug("sponsor logo failed:", err)\n'
        '\t\tc.Status(http.StatusNotFound)\n'
        '\t\treturn\n\t}\n'
        '\tc.Header("Cache-Control", "public, max-age=3600")\n'
        '\tc.Data(http.StatusOK, contentType, data)\n}\n',
    )
    for rel, tag in (
        ("frontend/src/pages/index/IndexPage.tsx", '<SponsorSlot slot="dashboard" />'),
        ("frontend/src/pages/login/LoginPage.tsx", '<SponsorSlot slot="login" variant="compact" />'),
    ):
        write(
            root, rel,
            "import SponsorSlot from '@/components/sponsor/SponsorSlot';\n"
            "export default function Page() {\n  return <>\n"
            f"    {tag}\n"
            "    <main>Keep dashboard/login content</main>\n  </>;\n}\n",
        )
    if layout == "legacy":
        write(
            root, "frontend/src/layouts/AppSidebar.tsx",
            "import {\n  CrownOutlined,\n} from '@ant-design/icons';\n"
            "import SponsorSlot from '@/components/sponsor/SponsorSlot';\n"
            "type IconName =\n  | 'sponsors'\n  | 'logout';\n"
            "const iconByName = {\n  sponsors: CrownOutlined,\n};\n"
            "const tabs = [\n"
            "      { key: '/sponsors', icon: 'sponsors', title: t('menu.sponsors') },\n"
            "];\n"
            "export const Sidebar = () => <>\n"
            "          <SponsorSlot\n            slot=\"sidebar\"\n"
            "            variant=\"compact\"\n            iconOnly={railCollapsed}\n"
            "            rotate\n            className=\"sider-sponsor\"\n          />\n"
            "          <SponsorSlot slot=\"sidebar\" variant=\"compact\" rotate className=\"sider-sponsor\" />\n"
            "          <div>Keep version info</div>\n</>;\n",
        )
        write(
            root, "frontend/src/routes.tsx",
            "const ApiDocsPage = lazy(() => import('@/pages/api-docs/ApiDocsPage'));\n"
            "const SponsorsPage = lazy(() => import('@/pages/sponsors/SponsorsPage'));\n"
            "export const routes = [\n"
            "      { path: 'api-docs', element: withSuspense(<ApiDocsPage />) },\n"
            "      { path: 'sponsors', element: withSuspense(<SponsorsPage />) },\n];\n",
        )
        write(
            root, "frontend/src/components/command-palette/CommandPalette.tsx",
            "import {\n  CrownOutlined,\n  ImportOutlined,\n} from '@ant-design/icons';\n"
            "const pages = [\n"
            "      {\n"
            "        path: '/sponsors',\n"
            "        title: t('menu.sponsors'),\n"
            "        keywords: ['sponsors', 'sponsor', 'partners'],\n"
            "        icon: <CrownOutlined />,\n"
            "      },\n"
            "];\n",
        )
    else:
        write(
            root, "frontend/src/layouts/AppSidebar.tsx",
            "export const Sidebar = () => <div>Keep standard sidebar</div>;\n",
        )
        write(
            root, "frontend/src/routes/panelRoutes.tsx",
            "import {\n  ApiOutlined,\n  CrownOutlined,\n} from '@ant-design/icons';\n"
            "const ApiDocsPage = lazy(() => import('@/pages/api-docs/ApiDocsPage'));\n"
            "const SponsorsPage = lazy(() => import('@/pages/sponsors/SponsorsPage'));\n"
            "export const PANEL_ROUTES = [\n"
            "  { path: '/api-docs', titleKey: 'menu.apiDocs', icon: ApiOutlined, component: ApiDocsPage },\n"
            "  { path: '/sponsors', titleKey: 'menu.sponsors', icon: CrownOutlined, component: SponsorsPage },\n"
            "];\n",
        )
        write(
            root, "frontend/src/components/command-palette/CommandPalette.tsx",
            "const keywords = {\n"
            "  '/api-docs': ['api'],\n"
            "  '/sponsors': ['sponsors', 'sponsor', 'partners'],\n"
            "};\n",
        )


def execute(root: Path):
    return subprocess.run(
        [sys.executable, str(SCRIPT), str(root)],
        text=True, capture_output=True, check=False,
    )


class SponsorRemovalTests(unittest.TestCase):
    def test_versions_with_no_sponsors_are_unchanged(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            write(root, "frontend/src/pages/index/IndexPage.tsx", "original")
            result = execute(root)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("skipped", result.stdout)
            self.assertEqual(read(root, "frontend/src/pages/index/IndexPage.tsx"), "original")

    def test_legacy_versions_remove_sidebar_and_navigation(self):
        self.assert_removed("legacy")

    def test_modern_version_removes_dashboard_login_and_navigation(self):
        self.assert_removed("modern")

    def assert_removed(self, layout: str):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            fixture(root, layout)
            result = execute(root)
            self.assertEqual(result.returncode, 0, result.stderr)
            for rel in (
                "frontend/src/pages/index/IndexPage.tsx",
                "frontend/src/pages/login/LoginPage.tsx",
                "frontend/src/layouts/AppSidebar.tsx",
                "frontend/src/components/command-palette/CommandPalette.tsx",
            ):
                self.assertNotIn("SponsorSlot", read(root, rel), rel)
                self.assertNotIn("'/sponsors'", read(root, rel), rel)
            routes = (
                "frontend/src/routes.tsx" if layout == "legacy"
                else "frontend/src/routes/panelRoutes.tsx"
            )
            self.assertNotIn("SponsorsPage", read(root, routes))
            self.assertNotIn("'/sponsors'", read(root, routes))
            controller = read(root, "internal/web/controller/index.go")
            self.assertIn("Sponsors: []panel.Sponsor{}", controller)
            self.assertIn("c.Status(http.StatusNotFound)", controller)
            self.assertNotIn("panelService.GetSponsors()", controller)
            self.assertNotIn("panelService.GetSponsorLogo(", controller)
            self.assertNotIn("internal/web/entity", controller)
            self.assertIn("Keep dashboard/login content", read(root, "frontend/src/pages/index/IndexPage.tsx"))
            self.assertIn("api-docs", read(root, routes))

    def test_upstream_anchor_drift_aborts(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            fixture(root, "modern")
            path = root / "frontend/src/pages/index/IndexPage.tsx"
            path.write_text(path.read_text().replace("SponsorSlot", "ChangedSlot"))
            result = execute(root)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("expected 1", result.stderr)


if __name__ == "__main__":
    unittest.main()
