"""
Benchmark configuration and system spec capture.

Automatically collects and prints machine specs at the start of any
benchmark run so results are reproducible and comparable.
"""
from __future__ import annotations

import os
import platform
import subprocess

import pytest


def _get_cpu_info() -> dict:
    """Collect CPU information."""
    info = {
        "processor": platform.processor() or "unknown",
        "arch": platform.machine(),
    }

    system = platform.system()
    if system == "Darwin":
        try:
            brand = subprocess.check_output(
                ["sysctl", "-n", "machdep.cpu.brand_string"], text=True
            ).strip()
            cores_physical = subprocess.check_output(
                ["sysctl", "-n", "hw.physicalcpu"], text=True
            ).strip()
            cores_logical = subprocess.check_output(
                ["sysctl", "-n", "hw.logicalcpu"], text=True
            ).strip()
            info["brand"] = brand
            info["cores_physical"] = int(cores_physical)
            info["cores_logical"] = int(cores_logical)
        except (subprocess.CalledProcessError, FileNotFoundError):
            pass
    elif system == "Linux":
        try:
            with open("/proc/cpuinfo") as f:
                for line in f:
                    if line.startswith("model name"):
                        info["brand"] = line.split(":")[1].strip()
                        break
            cores = os.cpu_count()
            info["cores_logical"] = cores or 0
        except FileNotFoundError:
            pass

    return info


def _get_memory_info() -> dict:
    """Collect memory information."""
    info = {}
    system = platform.system()

    if system == "Darwin":
        try:
            mem_bytes = subprocess.check_output(
                ["sysctl", "-n", "hw.memsize"], text=True
            ).strip()
            info["total_gb"] = round(int(mem_bytes) / (1024**3), 1)
        except (subprocess.CalledProcessError, FileNotFoundError):
            pass
    elif system == "Linux":
        try:
            with open("/proc/meminfo") as f:
                for line in f:
                    if line.startswith("MemTotal"):
                        kb = int(line.split()[1])
                        info["total_gb"] = round(kb / (1024**2), 1)
                        break
        except FileNotFoundError:
            pass

    return info


def _get_disk_info() -> dict:
    """Collect disk type information."""
    info = {}
    system = platform.system()

    if system == "Darwin":
        try:
            # Check if running on SSD
            output = subprocess.check_output(
                ["system_profiler", "SPNVMeDataType", "-detailLevel", "mini"],
                text=True,
                timeout=5,
            )
            if "NVMe" in output or "Apple" in output:
                info["type"] = "NVMe SSD"
            else:
                info["type"] = "SSD"
        except (subprocess.CalledProcessError, FileNotFoundError, subprocess.TimeoutExpired):
            info["type"] = "unknown"
    elif system == "Linux":
        try:
            # Check /sys/block for rotation
            for dev in os.listdir("/sys/block"):
                rotational_path = f"/sys/block/{dev}/queue/rotational"
                if os.path.exists(rotational_path):
                    with open(rotational_path) as f:
                        is_rotational = f.read().strip() == "1"
                    info["type"] = "HDD" if is_rotational else "SSD"
                    break
        except (OSError, FileNotFoundError):
            info["type"] = "unknown"

    return info


def _get_sqlite_version() -> str:
    """Get SQLite version."""
    import sqlite3
    return sqlite3.sqlite_version


def _collect_specs() -> dict:
    """Collect all system specs."""
    return {
        "os": f"{platform.system()} {platform.release()}",
        "os_version": platform.version(),
        "python": platform.python_version(),
        "sqlite": _get_sqlite_version(),
        "cpu": _get_cpu_info(),
        "memory": _get_memory_info(),
        "disk": _get_disk_info(),
    }


def _format_specs(specs: dict) -> str:
    """Format specs as a readable string."""
    cpu = specs["cpu"]
    mem = specs["memory"]
    disk = specs["disk"]

    lines = [
        "",
        "=" * 70,
        "BENCHMARK SYSTEM SPECS",
        "=" * 70,
        f"  OS:       {specs['os']}",
        f"  Python:   {specs['python']}",
        f"  SQLite:   {specs['sqlite']}",
    ]

    if "brand" in cpu:
        lines.append(f"  CPU:      {cpu['brand']}")
    if "cores_physical" in cpu:
        lines.append(f"  Cores:    {cpu['cores_physical']} physical / {cpu['cores_logical']} logical")
    elif "cores_logical" in cpu:
        lines.append(f"  Cores:    {cpu['cores_logical']} logical")

    if "total_gb" in mem:
        lines.append(f"  Memory:   {mem['total_gb']} GB")

    if "type" in disk:
        lines.append(f"  Disk:     {disk['type']}")

    lines.append("=" * 70)
    lines.append("")

    return "\n".join(lines)


@pytest.fixture(scope="session", autouse=True)
def print_system_specs():
    """Print system specs once at the start of the benchmark session."""
    specs = _collect_specs()
    print(_format_specs(specs))
    yield specs


def pytest_benchmark_update_machine_info(config, machine_info):
    """Hook: inject system specs into benchmark JSON output."""
    specs = _collect_specs()
    machine_info["system"] = specs["os"]
    machine_info["python"] = specs["python"]
    machine_info["sqlite"] = specs["sqlite"]

    cpu = specs["cpu"]
    if "brand" in cpu:
        machine_info["cpu_brand"] = cpu["brand"]
    if "cores_physical" in cpu:
        machine_info["cpu_cores_physical"] = cpu["cores_physical"]
        machine_info["cpu_cores_logical"] = cpu["cores_logical"]

    mem = specs["memory"]
    if "total_gb" in mem:
        machine_info["memory_gb"] = mem["total_gb"]

    disk = specs["disk"]
    if "type" in disk:
        machine_info["disk_type"] = disk["type"]
