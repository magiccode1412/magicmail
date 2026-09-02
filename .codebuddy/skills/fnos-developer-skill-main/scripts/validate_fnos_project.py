#!/usr/bin/env python3
"""Heuristic validator for a flyNAS fnOS application project.

This tool checks common package structure, configuration, API scope, gateway,
and security mistakes. It does not replace `fnpack build` or device testing.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import stat
import struct
import sys
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any, Iterable

SEVERITY_RANK = {"blocker": 0, "high": 1, "medium": 2, "low": 3, "info": 4}
KNOWN_SCOPES = {
    "trim.file.sharedAccess",
    "trim.file.userAccess",
    "trim.file.userAcl",
    "trim.file.path",
    "trim.system.getPlatformConfig",
}
TEXT_SUFFIXES = {
    "", ".sh", ".bash", ".js", ".mjs", ".cjs", ".ts", ".tsx", ".jsx",
    ".json", ".html", ".htm", ".css", ".py", ".java", ".go", ".rs",
    ".yaml", ".yml", ".toml", ".ini", ".conf", ".cgi", ".md",
}
MAX_SCAN_BYTES = 2 * 1024 * 1024


@dataclass(frozen=True)
class Issue:
    severity: str
    code: str
    path: str
    message: str


class Validator:
    def __init__(self, root: Path) -> None:
        self.root = root.resolve()
        self.issues: list[Issue] = []
        self.manifest: dict[str, str] = {}
        self.privilege: dict[str, Any] = {}
        self.resource: dict[str, Any] = {}
        self.ui_config: dict[str, Any] = {}
        self.text_files: dict[Path, str] = {}

    def add(self, severity: str, code: str, path: Path | str, message: str) -> None:
        if isinstance(path, Path):
            try:
                display = str(path.relative_to(self.root))
            except ValueError:
                display = str(path)
        else:
            display = path
        self.issues.append(Issue(severity, code, display, message))

    def run(self) -> list[Issue]:
        if not self.root.is_dir():
            self.add("blocker", "project.not_directory", self.root, "项目路径不存在或不是目录。")
            return self.issues

        self._scan_text_files()
        self._check_required_structure()
        self._check_manifest()
        self._check_json_configs()
        self._check_privilege()
        self._check_resource_and_scopes()
        self._check_ui_config()
        self._check_lifecycle_scripts()
        self._check_icons()
        self._check_sdk_and_api_usage()
        self._check_security_patterns()

        self.issues.sort(key=lambda i: (SEVERITY_RANK[i.severity], i.path, i.code))
        return self.issues

    def _scan_text_files(self) -> None:
        for path in self.root.rglob("*"):
            if not path.is_file() or path.is_symlink():
                continue
            try:
                if path.stat().st_size > MAX_SCAN_BYTES:
                    continue
            except OSError:
                continue
            if path.suffix.lower() not in TEXT_SUFFIXES and path.name not in {
                "manifest", "privilege", "resource", "config", "main",
                "install_init", "install_callback", "upgrade_init",
                "upgrade_callback", "uninstall_init", "uninstall_callback",
                "config_init", "config_callback", "install", "upgrade",
                "uninstall",
            }:
                continue
            try:
                self.text_files[path] = path.read_text(encoding="utf-8")
            except (UnicodeDecodeError, OSError):
                continue

    def _check_required_structure(self) -> None:
        required_files = [
            self.root / "manifest",
            self.root / "config" / "privilege",
            self.root / "config" / "resource",
            self.root / "ICON.PNG",
            self.root / "ICON_256.PNG",
        ]
        required_dirs = [self.root / "app", self.root / "cmd", self.root / "wizard"]
        for path in required_files:
            if not path.is_file():
                self.add("blocker", "structure.missing_file", path, "fnpack 基础结构缺少必需文件。")
        for path in required_dirs:
            if not path.is_dir():
                self.add("blocker", "structure.missing_dir", path, "fnpack 基础结构缺少必需目录。")

    @staticmethod
    def _parse_manifest(text: str) -> tuple[dict[str, str], list[str]]:
        result: dict[str, str] = {}
        errors: list[str] = []
        for number, raw in enumerate(text.splitlines(), 1):
            line = raw.strip()
            if not line or line.startswith("#"):
                continue
            if "=" not in line:
                errors.append(f"第 {number} 行缺少 '='")
                continue
            key, value = line.split("=", 1)
            key = key.strip()
            value = value.strip()
            if not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", key):
                errors.append(f"第 {number} 行字段名非法: {key!r}")
                continue
            if len(value) >= 2 and value[0] == value[-1] and value[0] in {'"', "'"}:
                value = value[1:-1]
            if key in result:
                errors.append(f"第 {number} 行字段重复: {key}")
            result[key] = value
        return result, errors

    def _check_manifest(self) -> None:
        path = self.root / "manifest"
        text = self.text_files.get(path)
        if text is None:
            return
        self.manifest, errors = self._parse_manifest(text)
        for error in errors:
            self.add("high", "manifest.syntax", path, error)

        required = ["appname", "version", "display_name", "source", "platform"]
        for field in required:
            if not self.manifest.get(field):
                self.add("blocker", "manifest.missing_field", path, f"缺少基础字段 {field}。")

        appname = self.manifest.get("appname", "")
        if appname and not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", appname):
            self.add("medium", "manifest.appname_format", path, "appname 含非常规字符；确认与系统命名规则和路径一致。")
        if self.manifest.get("source") and self.manifest.get("source") != "thirdparty":
            self.add("medium", "manifest.source", path, "第三方应用通常应使用 source=thirdparty。")
        platform = self.manifest.get("platform")
        if platform and platform not in {"x86", "arm", "all"}:
            self.add("high", "manifest.platform", path, "platform 应为 x86、arm 或 all。")
        if platform == "all" and self._has_native_binaries():
            self.add("high", "manifest.platform_all_native", path, "发现疑似原生二进制，但 platform=all；请按目标架构构建并声明。")

        desktop_uidir = self.manifest.get("desktop_uidir")
        if desktop_uidir:
            ui_dir = self.root / "app" / desktop_uidir
            if not ui_dir.is_dir():
                self.add("blocker", "manifest.desktop_uidir_missing", ui_dir, "manifest 声明的 desktop_uidir 目录不存在。")
            elif not (ui_dir / "config").is_file():
                self.add("high", "ui.config_missing", ui_dir / "config", "桌面 UI 目录缺少入口 config。")

        ctl_stop = self.manifest.get("ctl_stop", "").lower()
        if ctl_stop != "false" and not (self.root / "cmd" / "main").is_file():
            self.add("high", "manifest.ctl_without_main", path, "应用保留运行控制，但缺少 cmd/main。")

    def _has_native_binaries(self) -> bool:
        app_dir = self.root / "app"
        if not app_dir.is_dir():
            return False
        for path in app_dir.rglob("*"):
            if not path.is_file() or path.is_symlink():
                continue
            try:
                head = path.read_bytes()[:4]
            except OSError:
                continue
            if head == b"\x7fELF" or head[:2] == b"MZ":
                return True
        return False

    def _load_json(self, path: Path, severity: str = "blocker") -> dict[str, Any] | list[Any] | None:
        text = self.text_files.get(path)
        if text is None:
            return None
        try:
            value = json.loads(text)
        except json.JSONDecodeError as exc:
            self.add(severity, "json.invalid", path, f"JSON 解析失败：第 {exc.lineno} 行第 {exc.colno} 列。")
            return None
        return value

    def _check_json_configs(self) -> None:
        privilege_path = self.root / "config" / "privilege"
        resource_path = self.root / "config" / "resource"
        privilege = self._load_json(privilege_path)
        resource = self._load_json(resource_path)
        if isinstance(privilege, dict):
            self.privilege = privilege
        elif privilege is not None:
            self.add("blocker", "privilege.not_object", privilege_path, "config/privilege 顶层必须是 JSON 对象。")
        if isinstance(resource, dict):
            self.resource = resource
        elif resource is not None:
            self.add("blocker", "resource.not_object", resource_path, "config/resource 顶层必须是 JSON 对象。")

        desktop_uidir = self.manifest.get("desktop_uidir")
        if desktop_uidir:
            ui_path = self.root / "app" / desktop_uidir / "config"
            ui = self._load_json(ui_path, severity="high")
            if isinstance(ui, dict):
                self.ui_config = ui
            elif ui is not None:
                self.add("high", "ui.not_object", ui_path, "app UI config 顶层必须是 JSON 对象。")

        for wizard_name in ("install", "upgrade", "uninstall", "config"):
            path = self.root / "wizard" / wizard_name
            if not path.is_file():
                continue
            value = self._load_json(path, severity="high")
            if value is not None and not isinstance(value, list):
                self.add("high", "wizard.not_array", path, "向导文件顶层必须是步骤数组。")

    def _check_privilege(self) -> None:
        if not self.privilege:
            return
        path = self.root / "config" / "privilege"
        defaults = self.privilege.get("defaults")
        if not isinstance(defaults, dict):
            self.add("high", "privilege.defaults", path, "缺少 defaults 对象。")
            return
        run_as = defaults.get("run-as")
        if run_as == "root":
            self.add("high", "privilege.root", path, "应用配置为 root；必须证明无更窄方案，并让长期服务降权。")
        elif run_as != "package":
            self.add("medium", "privilege.run_as", path, "大多数应用应使用 defaults.run-as=package。")

        groups = self.privilege.get("join-groups", [])
        if groups and not isinstance(groups, list):
            self.add("high", "privilege.join_groups_type", path, "join-groups 应为字符串数组。")
        elif isinstance(groups, list):
            for group in groups:
                if not isinstance(group, str) or not group:
                    self.add("high", "privilege.join_groups_value", path, "join-groups 含非法值。")
            if groups:
                self.add("low", "privilege.join_groups_review", path, f"确认附加用户组均为必要项：{groups!r}。")

    def _check_resource_and_scopes(self) -> None:
        if not self.resource:
            return
        path = self.root / "config" / "resource"
        scopes = self.resource.get("api-scope", [])
        if scopes and not isinstance(scopes, list):
            self.add("blocker", "scope.not_array", path, "api-scope 必须是字符串数组。")
            return
        if isinstance(scopes, list):
            seen: set[str] = set()
            for scope in scopes:
                if not isinstance(scope, str):
                    self.add("high", "scope.non_string", path, "api-scope 含非字符串值。")
                    continue
                if scope in seen:
                    self.add("low", "scope.duplicate", path, f"重复 Scope：{scope}。")
                seen.add(scope)
                if scope not in KNOWN_SCOPES:
                    self.add("medium", "scope.unknown", path, f"当前 Skill 未收录 Scope：{scope}；请核对最新官方文档。")

        shares = self.resource.get("data-share")
        if isinstance(shares, dict):
            items = shares.get("shares", [])
            if not isinstance(items, list):
                self.add("high", "resource.shares_type", path, "data-share.shares 应为数组。")
            else:
                names: set[str] = set()
                for item in items:
                    if not isinstance(item, dict) or not isinstance(item.get("name"), str):
                        self.add("high", "resource.share_item", path, "共享目录项必须包含字符串 name。")
                        continue
                    name = item["name"]
                    if name.startswith("/") or ".." in name.split("/"):
                        self.add("high", "resource.share_name", path, f"共享目录名称不应是绝对路径或包含 '..'：{name}。")
                    if name in names:
                        self.add("medium", "resource.share_duplicate", path, f"共享目录重复：{name}。")
                    names.add(name)

        docker = self.resource.get("docker-project")
        if isinstance(docker, dict):
            projects = docker.get("projects", [])
            if isinstance(projects, list):
                for project in projects:
                    if not isinstance(project, dict):
                        continue
                    rel = project.get("path")
                    if isinstance(rel, str):
                        compose_dir = self.root / "app" / rel
                        if not (compose_dir / "docker-compose.yaml").is_file():
                            self.add("high", "docker.compose_missing", compose_dir / "docker-compose.yaml", "docker-project 指向目录缺少 docker-compose.yaml。")

    def _check_ui_config(self) -> None:
        if not self.ui_config:
            return
        desktop_uidir = self.manifest.get("desktop_uidir", "ui")
        path = self.root / "app" / desktop_uidir / "config"
        urls = self.ui_config.get(".url")
        if not isinstance(urls, dict) or not urls:
            self.add("high", "ui.urls_missing", path, "入口配置缺少非空 .url 对象。")
            return

        launch = self.manifest.get("desktop_applaunchname")
        if launch and launch not in urls:
            self.add("high", "ui.launch_missing", path, "desktop_applaunchname 未匹配任何入口 ID。")

        appname = self.manifest.get("appname", "")
        for entry_id, entry in urls.items():
            if not isinstance(entry, dict):
                self.add("high", "ui.entry_type", path, f"入口 {entry_id} 必须是对象。")
                continue
            if appname and not str(entry_id).lower().startswith(appname.lower()):
                self.add("low", "ui.entry_prefix", path, f"入口 ID {entry_id} 建议使用 appname 前缀。")
            if entry.get("type") not in {"iframe", "url"}:
                self.add("medium", "ui.entry_type_value", path, f"入口 {entry_id} 的 type 应为 iframe 或 url。")

            prefix = entry.get("gatewayPrefix")
            socket = entry.get("gatewaySocket")
            if prefix is not None or socket is not None:
                if not isinstance(prefix, str) or not prefix.startswith("/app/"):
                    self.add("high", "gateway.prefix", path, f"入口 {entry_id} 的 gatewayPrefix 应以 /app/ 开头。")
                if isinstance(prefix, str) and "." in prefix:
                    self.add("medium", "gateway.prefix_dot", path, f"入口 {entry_id} 的公开网关路径应避免点号。")
                if not isinstance(socket, str) or not socket or "/" in socket or "\\" in socket:
                    self.add("high", "gateway.socket", path, f"入口 {entry_id} 的 gatewaySocket 只应填写 Socket 文件名。")
                if entry.get("url") != prefix:
                    self.add("medium", "gateway.url_mismatch", path, f"入口 {entry_id} 的 url 通常应与 gatewayPrefix 一致。")

            icon = entry.get("icon")
            if isinstance(icon, str) and "{0}" in icon:
                for size in (64, 256):
                    icon_path = self.root / "app" / desktop_uidir / icon.replace("{0}", str(size))
                    if not icon_path.is_file():
                        self.add("medium", "ui.icon_missing", icon_path, f"入口 {entry_id} 引用的 {size}px 图标不存在。")

            file_types = entry.get("fileTypes")
            if file_types is not None and not (
                isinstance(file_types, list) and all(isinstance(v, str) and v for v in file_types)
            ):
                self.add("high", "ui.file_types", path, f"入口 {entry_id} 的 fileTypes 应为非空字符串数组。")

    def _check_lifecycle_scripts(self) -> None:
        cmd_dir = self.root / "cmd"
        if not cmd_dir.is_dir():
            return
        names = [
            "main", "install_init", "install_callback", "upgrade_init",
            "upgrade_callback", "uninstall_init", "uninstall_callback",
            "config_init", "config_callback",
        ]
        for name in names:
            path = cmd_dir / name
            if not path.is_file():
                continue
            try:
                mode = path.stat().st_mode
                if not mode & stat.S_IXUSR:
                    self.add("medium", "script.not_executable", path, "生命周期脚本未设置用户可执行位。")
            except OSError:
                pass

        main = cmd_dir / "main"
        text = self.text_files.get(main, "")
        if text:
            if "status" not in text:
                self.add("high", "main.status_missing", main, "cmd/main 未明显处理 status。")
            if not re.search(r"exit\s+3\b", text):
                self.add("medium", "main.status_code", main, "未发现 status 未运行时返回 3 的逻辑。")
            if "/var/apps/" in text:
                self.add("medium", "main.hardcoded_app_path", main, "cmd/main 硬编码 /var/apps 路径；应优先使用 TRIM_* 环境变量。")

    @staticmethod
    def _png_dimensions(path: Path) -> tuple[int, int] | None:
        try:
            with path.open("rb") as handle:
                header = handle.read(24)
        except OSError:
            return None
        if len(header) >= 24 and header[:8] == b"\x89PNG\r\n\x1a\n":
            return struct.unpack(">II", header[16:24])
        return None

    def _check_icons(self) -> None:
        expected = {"ICON.PNG": (64, 64), "ICON_256.PNG": (256, 256)}
        for name, dims in expected.items():
            path = self.root / name
            if not path.is_file():
                continue
            try:
                if path.stat().st_size > 1024 * 1024:
                    self.add("medium", "icon.too_large", path, "图标文件超过 1024 KB。")
            except OSError:
                pass
            actual = self._png_dimensions(path)
            if actual and actual != dims:
                self.add("medium", "icon.dimensions", path, f"PNG 尺寸为 {actual[0]}x{actual[1]}，期望 {dims[0]}x{dims[1]}。")

    def _all_text(self) -> str:
        return "\n".join(self.text_files.values())

    def _check_sdk_and_api_usage(self) -> None:
        all_text = self._all_text()
        manifest_path = self.root / "manifest"
        resource_path = self.root / "config" / "resource"
        scopes = set(self.resource.get("api-scope", [])) if isinstance(self.resource.get("api-scope", []), list) else set()

        uses_sdk = "@trimjs/web-app" in all_text or "new TrimApp" in all_text
        if uses_sdk and self.manifest.get("micro_app", "").lower() != "true":
            self.add("high", "sdk.micro_app", manifest_path, "检测到 TrimApp JS SDK，但 manifest 未声明 micro_app=true。")

        requirements = {
            "trim.file.sharedAccess": ["pickSharedFile", "authorizeSharedFile", "trim.file.getSharedAccessibleFolders", "trim.file.delSharedAccessibleFolder"],
            "trim.file.userAccess": ["pickUserFile", "authorizeUserFile", "trim.file.getUserAccessibleFolders", "trim.file.delUserAccessibleFolder"],
            "trim.file.userAcl": ["trim.file.checkUserACL"],
            "trim.file.path": ["trim.file.convertPath"],
            "trim.system.getPlatformConfig": ["trim.system.getPlatformConfig"],
        }
        for scope, markers in requirements.items():
            if any(marker in all_text for marker in markers) and scope not in scopes:
                self.add("high", "scope.missing_for_usage", resource_path, f"代码使用 {markers[0]} 等能力，但未声明 Scope {scope}。")

        if "openAppAuth" in all_text:
            if "parseAppAuthCallback" not in all_text:
                self.add("medium", "auth.callback_missing", self.root, "使用 openAppAuth，但未发现 parseAppAuthCallback 回调处理。")
            if "state" not in all_text:
                self.add("high", "auth.state_missing", self.root, "使用 openAppAuth，但未发现业务 state 生成/校验。")
            if "postMessage" in all_text and "event.origin" not in all_text:
                self.add("high", "auth.origin_check", self.root, "使用 postMessage 回传授权结果，但未发现 event.origin 校验。")

        if "$on('os/theme'" in all_text or '$on("os/theme"' in all_text or "$on('os/language'" in all_text or '$on("os/language"' in all_text:
            if "isStandaloneWeb" not in all_text or "isWeb" not in all_text:
                self.add("medium", "sdk.on_environment", self.root, "$on 事件监听应限制为 Web 宿主且非独立浏览器环境。")

    def _check_security_patterns(self) -> None:
        frontend_roots = [self.root / "app" / "ui", self.root / "app" / "www"]
        for path, text in self.text_files.items():
            normalized = text.lower()
            is_frontend = any(root == path or root in path.parents for root in frontend_roots)
            if is_frontend and ("trim_api_token" in normalized or "/api/v1/trimapp" in normalized):
                self.add("blocker", "token.frontend", path, "前端文件包含 TRIM_API_TOKEN 或后端 API 路径；后端 API 不得从浏览器调用。")

            if re.search(r"(?:echo|console\.log|print)\s*\(?[^\n]*TRIM_API_TOKEN", text, re.IGNORECASE):
                self.add("blocker", "token.logging", path, "发现可能把 TRIM_API_TOKEN 输出到日志或终端的代码。")

            if re.search(r"TRIM_API_TOKEN[^\n]*(?:>|>>|writeFile|write_text|database|insert)", text, re.IGNORECASE):
                self.add("blocker", "token.persistence", path, "发现可能持久化 TRIM_API_TOKEN 的代码。")

            if re.search(r"(?:req\.(?:query|body)|message)[^\n]{0,120}(?:uid|userid|isadmin)", normalized):
                self.add("high", "identity.client_controlled", path, "发现可能从客户端输入读取 UID/管理员标记；统一网关应用应使用 X-Trim-* Header。")

            if "child_process.exec(" in text or re.search(r"\bos\.system\s*\(", text):
                self.add("high", "command.shell_exec", path, "发现 shell 字符串执行；确认没有拼接用户输入，优先使用参数数组。")

            if path.name == "index.cgi" and ("REQUEST_URI" in text or "PATH_INFO" in text):
                if "realpath" not in text and "readlink -f" not in text:
                    self.add("medium", "cgi.realpath_boundary", path, "CGI 映射请求路径但未发现真实路径边界校验；仅检查 '..' 不足以防符号链接逃逸。")

        gateway_sources = [text for text in self.text_files.values() if "X-Trim-" in text or "x-trim-" in text.lower()]
        if gateway_sources:
            combined = "\n".join(gateway_sources).lower()
            if "x-trim-userid" not in combined:
                self.add("medium", "gateway.uid_missing", self.root, "使用网关身份 Header，但未发现 X-Trim-Userid。")


def format_text(root: Path, issues: Iterable[Issue]) -> str:
    issues = list(issues)
    counts = {name: 0 for name in SEVERITY_RANK}
    for issue in issues:
        counts[issue.severity] += 1
    lines = [
        f"fnOS project heuristic validation: {root}",
        "NOTE: This does not replace fnpack build or testing on an fnOS device.",
        "",
    ]
    if not issues:
        lines.append("No issues found by this heuristic validator.")
        return "\n".join(lines)
    for issue in issues:
        lines.append(f"[{issue.severity.upper():7}] {issue.code} | {issue.path} | {issue.message}")
    lines.extend([
        "",
        "Summary: " + ", ".join(f"{k}={v}" for k, v in counts.items() if v),
    ])
    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("project", type=Path, help="fnOS application project directory")
    parser.add_argument("--json", action="store_true", help="emit JSON")
    args = parser.parse_args()

    validator = Validator(args.project)
    issues = validator.run()
    if args.json:
        payload = {
            "project": str(validator.root),
            "note": "Heuristic only; run fnpack build and test on an fnOS device.",
            "issues": [asdict(issue) for issue in issues],
        }
        print(json.dumps(payload, ensure_ascii=False, indent=2))
    else:
        print(format_text(validator.root, issues))

    return 1 if any(issue.severity in {"blocker", "high"} for issue in issues) else 0


if __name__ == "__main__":
    sys.exit(main())
