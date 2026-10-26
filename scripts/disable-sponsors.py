#!/usr/bin/env python3
"""Disable upstream 3x-ui sponsor placements without touching protocol services.

v3.7/v3.8 have no sponsor module and are intentionally left unchanged.
v3.9/v3.10 have a sidebar placement and a legacy route map.
v3.11 has a centralized panel route list and no sidebar placement.
"""
from pathlib import Path
import re
import sys

if len(sys.argv) != 2:
    raise SystemExit(f"Usage: {sys.argv[0]} /path/to/3x-ui-source")

root = Path(sys.argv[1]).resolve()


def edit(rel: str, callback) -> None:
    path = root / rel
    original = path.read_text(encoding="utf-8")
    updated = callback(original)
    if updated != original:
        path.write_text(updated, encoding="utf-8")


def remove_literal(rel: str, original: str, anchor: str) -> None:
    def apply(text: str) -> str:
        count = text.count(original)
        if count != 1:
            raise SystemExit(f"remove sponsors: {anchor} in {rel}: expected 1, found {count}")
        return text.replace(original, "", 1)
    edit(rel, apply)


def remove_regex(rel: str, pattern: str, anchor: str, expected: int = 1) -> None:
    def apply(text: str) -> str:
        updated, count = re.subn(pattern, "", text, flags=re.MULTILINE)
        if count != expected:
            raise SystemExit(
                f"remove sponsors: {anchor} in {rel}: expected {expected}, found {count}"
            )
        return updated
    edit(rel, apply)


def replace_method(rel: str, method: str, replacement: str, required_call: str) -> None:
    def apply(text: str) -> str:
        pat = (
            r"^func \(a \*IndexController\) "
            + method
            + r"\(c \*gin\.Context\) \{\n.*?^\}\n"
        )
        matches = list(re.finditer(pat, text, flags=re.MULTILINE | re.DOTALL))
        if len(matches) != 1:
            raise SystemExit(f"remove sponsors: missing or ambiguous Go method {method}")
        match = matches[0]
        if required_call not in match.group():
            raise SystemExit(f"remove sponsors: unexpected Go method body {method}")
        return text[:match.start()] + replacement + text[match.end():]
    edit(rel, apply)


def main() -> None:
    sponsor_file = root / "internal/web/service/panel/sponsor.go"
    sponsor_slot = root / "frontend/src/components/sponsor/SponsorSlot.tsx"
    if not sponsor_file.exists() and not sponsor_slot.exists():
        print("remove sponsors: no sponsor feature in this upstream version; skipped")
        return
    if not sponsor_file.exists() or not sponsor_slot.exists():
        raise SystemExit("remove sponsors: partial upstream sponsor feature detected")

    # Remove all visual placements, including the two legacy sidebar slots.
    for rel in (
        "frontend/src/pages/index/IndexPage.tsx",
        "frontend/src/pages/login/LoginPage.tsx",
    ):
        remove_regex(rel, r"^[ \t]*<SponsorSlot\b[^>]*?/>[ \t]*\n", "SponsorSlot markup")
        remove_literal(
            rel,
            "import SponsorSlot from '@/components/sponsor/SponsorSlot';\n",
            "SponsorSlot import",
        )

    sidebar = "frontend/src/layouts/AppSidebar.tsx"
    sidebar_text = (root / sidebar).read_text(encoding="utf-8")
    if "<SponsorSlot" in sidebar_text:
        remove_regex(
            sidebar, r"^[ \t]*<SponsorSlot\b[^>]*?/>[ \t]*\n",
            "legacy sidebar placements", expected=2,
        )
        remove_literal(
            sidebar,
            "import SponsorSlot from '@/components/sponsor/SponsorSlot';\n",
            "legacy sidebar slot import",
        )
        remove_literal(sidebar, "      { key: '/sponsors', icon: 'sponsors', title: t('menu.sponsors') },\n", "legacy sidebar menu")
        remove_literal(sidebar, "  | 'sponsors'\n", "legacy sidebar icon type")
        remove_literal(sidebar, "  sponsors: CrownOutlined,\n", "legacy sidebar icon mapping")
        remove_literal(sidebar, "  CrownOutlined,\n", "legacy CrownOutlined import")
    elif "SponsorSlot" in sidebar_text:
        raise SystemExit("remove sponsors: unexpected sidebar SponsorSlot implementation")

    # Old and new 3x-ui releases register their pages differently.
    routes_new = "frontend/src/routes/panelRoutes.tsx"
    if (root / routes_new).exists():
        remove_literal(
            routes_new,
            "const SponsorsPage = lazy(() => import('@/pages/sponsors/SponsorsPage'));\n",
            "sponsors page import",
        )
        remove_literal(
            routes_new,
            "  { path: '/sponsors', titleKey: 'menu.sponsors', icon: CrownOutlined, component: SponsorsPage },\n",
            "sponsors route",
        )
        remove_literal(routes_new, "  CrownOutlined,\n", "route CrownOutlined import")
        remove_literal(
            "frontend/src/components/command-palette/CommandPalette.tsx",
            "  '/sponsors': ['sponsors', 'sponsor', 'partners'],\n",
            "sponsors command shortcut",
        )
    else:
        legacy_routes = "frontend/src/routes.tsx"
        remove_literal(
            legacy_routes,
            "const SponsorsPage = lazy(() => import('@/pages/sponsors/SponsorsPage'));\n",
            "legacy sponsors page import",
        )
        remove_literal(
            legacy_routes,
            "      { path: 'sponsors', element: withSuspense(<SponsorsPage />) },\n",
            "legacy sponsors route",
        )
        palette = "frontend/src/components/command-palette/CommandPalette.tsx"
        remove_literal(
            palette,
            "      {\n"
            "        path: '/sponsors',\n"
            "        title: t('menu.sponsors'),\n"
            "        keywords: ['sponsors', 'sponsor', 'partners'],\n"
            "        icon: <CrownOutlined />,\n"
            "      },\n",
            "legacy sponsors command entry",
        )
        remove_literal(palette, "  CrownOutlined,\n", "legacy palette CrownOutlined import")

    # HTTP responses stay API-compatible. No request may reach the remote
    # sponsors list or logo fetchers via the public panel routes.
    controller = "internal/web/controller/index.go"
    replace_method(
        controller,
        "sponsors",
        "func (a *IndexController) sponsors(c *gin.Context) {\n"
        "\tjsonObj(c, &panel.SponsorList{Sponsors: []panel.Sponsor{}}, nil)\n"
        "}\n",
        "a.panelService.GetSponsors()",
    )
    replace_method(
        controller,
        "sponsorLogo",
        "func (a *IndexController) sponsorLogo(c *gin.Context) {\n"
        "\tc.Status(http.StatusNotFound)\n"
        "}\n",
        "a.panelService.GetSponsorLogo(",
    )
    remove_literal(
        controller,
        '\t"github.com/mhsanaei/3x-ui/v3/internal/web/entity"\n',
        "now-unused entity import",
    )
    print("remove sponsors: dashboard/login/sidebar, navigation and remote sponsor HTTP fetches disabled")


if __name__ == "__main__":
    main()
