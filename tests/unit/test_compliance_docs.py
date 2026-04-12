"""Tests that verify the compliance documentation suite exists and is
well-formed.

This is documentation-surface coverage rather than runtime behaviour:
we want CI to break if any compliance document is accidentally deleted
or truncated below its minimum useful length, and we want to make sure
the supporting evidence collection script stays executable.
"""

from __future__ import annotations

import re
import stat
from pathlib import Path

import pytest

REPO_ROOT = Path(__file__).resolve().parents[2]
COMPLIANCE_DIR = REPO_ROOT / "docs" / "compliance"
SCRIPTS_DIR = REPO_ROOT / "scripts"


# Minimum line counts per file. Enforced loosely so small editorial
# tweaks don't cause false failures, but large deletions will.
REQUIRED_DOCS: dict[str, int] = {
    "soc2-evidence.md": 200,
    "iso27001-isms.md": 300,
    "baa-template.md": 150,
    "privacy-policy.md": 200,
    "incident-response.md": 250,
    "business-continuity.md": 200,
    "README.md": 40,
}


def _line_count(path: Path) -> int:
    with path.open("r", encoding="utf-8") as fh:
        return sum(1 for _ in fh)


@pytest.mark.unit
class TestComplianceDocs:
    """Compliance documentation presence and sanity checks."""

    def test_compliance_dir_exists(self):
        """docs/compliance/ exists."""
        assert COMPLIANCE_DIR.is_dir(), f"missing {COMPLIANCE_DIR}"

    @pytest.mark.parametrize("filename", sorted(REQUIRED_DOCS.keys()))
    def test_required_doc_exists(self, filename: str):
        """Each required compliance doc file exists."""
        path = COMPLIANCE_DIR / filename
        assert path.is_file(), f"missing compliance doc: {filename}"

    @pytest.mark.parametrize(
        "filename,min_lines",
        sorted(REQUIRED_DOCS.items()),
    )
    def test_required_doc_has_min_length(self, filename: str, min_lines: int):
        """Each required compliance doc meets its minimum line count."""
        path = COMPLIANCE_DIR / filename
        actual = _line_count(path)
        assert actual >= min_lines, (
            f"{filename} has only {actual} lines, expected >= {min_lines}"
        )

    def test_readme_links_resolve(self):
        """All relative markdown links in the compliance README resolve."""
        readme = COMPLIANCE_DIR / "README.md"
        text = readme.read_text(encoding="utf-8")
        # Match only relative links starting with ./ or ../ to avoid
        # remote URL flakiness.
        link_pattern = re.compile(r"\]\((\.\.?/[^)\s#]+)")
        missing: list[str] = []
        for match in link_pattern.finditer(text):
            rel = match.group(1)
            target = (readme.parent / rel).resolve()
            if not target.exists():
                missing.append(rel)
        assert not missing, f"broken README links: {missing}"

    def test_soc2_evidence_mentions_trust_service_criteria(self):
        """SOC 2 evidence guide references the CC families."""
        text = (COMPLIANCE_DIR / "soc2-evidence.md").read_text(encoding="utf-8")
        for cc in ("CC1", "CC6", "CC7", "CC8"):
            assert cc in text, f"soc2-evidence.md missing section {cc}"

    def test_iso27001_lists_annex_a_controls(self):
        """ISMS document references Annex A controls."""
        text = (COMPLIANCE_DIR / "iso27001-isms.md").read_text(encoding="utf-8")
        # A minimum set of critical Annex A controls we expect mapped.
        for control in ("A.5.15", "A.8.3", "A.8.15", "A.8.24"):
            assert control in text, f"iso27001-isms.md missing {control}"

    def test_incident_response_has_severity_levels(self):
        """Incident response doc defines P1-P4 severities with timelines."""
        text = (COMPLIANCE_DIR / "incident-response.md").read_text(encoding="utf-8")
        for sev in ("P1", "P2", "P3", "P4"):
            assert sev in text, f"incident-response.md missing {sev}"
        # GDPR 72 hour and HIPAA 60 day windows should both be present.
        assert "72 hours" in text or "72 hour" in text
        assert "60 days" in text or "60 calendar days" in text

    def test_business_continuity_has_rto_rpo_tiers(self):
        """BCP/DR doc defines the three recovery tiers with RTO/RPO."""
        text = (COMPLIANCE_DIR / "business-continuity.md").read_text(encoding="utf-8")
        assert "Tier 1" in text
        assert "Tier 2" in text
        assert "Tier 3" in text
        assert "RTO" in text
        assert "RPO" in text
        assert "5 minutes" in text or "5 min" in text
        assert "1 hour" in text or "1 hr" in text
        assert "4 hours" in text or "4 hr" in text

    def test_baa_template_has_fillable_placeholders(self):
        """BAA template exposes [CUSTOMER NAME] and other placeholders."""
        text = (COMPLIANCE_DIR / "baa-template.md").read_text(encoding="utf-8")
        assert "[CUSTOMER NAME]" in text
        assert "Business Associate" in text
        assert "Subcontractor" in text or "subcontractor" in text

    def test_privacy_policy_has_gdpr_and_ccpa(self):
        """Privacy policy references both GDPR and CCPA rights."""
        text = (COMPLIANCE_DIR / "privacy-policy.md").read_text(encoding="utf-8")
        assert "GDPR" in text
        assert "CCPA" in text or "California Consumer Privacy Act" in text
        # Articles 13-14 coverage is required for the data-subject rights test.
        assert "13" in text and "14" in text

    def test_evidence_script_exists_and_executable(self):
        """scripts/collect_soc2_evidence.py is present and executable."""
        script = SCRIPTS_DIR / "collect_soc2_evidence.py"
        assert script.is_file(), f"missing {script}"
        mode = script.stat().st_mode
        assert mode & stat.S_IXUSR, f"{script} is not executable"
        # Must have a shebang so it can be run directly.
        first_line = script.read_text(encoding="utf-8").splitlines()[0]
        assert first_line.startswith("#!"), f"{script} missing shebang"
