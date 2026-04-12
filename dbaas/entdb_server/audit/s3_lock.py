"""S3 Object Lock configuration for WAL compliance.

S3 Object Lock in Compliance mode provides WORM (Write Once Read Many)
protection that satisfies SOC 2, HIPAA, and GDPR audit trail requirements.
Not even the root AWS account can delete locked objects.
"""

from __future__ import annotations

import logging
from dataclasses import dataclass
from typing import Any

logger = logging.getLogger(__name__)


@dataclass(frozen=True)
class S3ObjectLockConfig:
    """Configuration for S3 Object Lock on the WAL bucket.

    Attributes:
        mode: Lock mode — "COMPLIANCE" (immutable, recommended) or
              "GOVERNANCE" (deletable with special permission).
        retention_days: How long objects are locked. Set per regulation:
              HIPAA=2190 (6yr), SOC2=365, GDPR=varies.
        enable_cloudtrail: Whether to enable CloudTrail logging on the
              bucket for access auditing.
    """

    mode: str = "COMPLIANCE"
    retention_days: int = 2190
    enable_cloudtrail: bool = True


async def configure_s3_object_lock(
    s3_client: Any,
    bucket: str,
    config: S3ObjectLockConfig | None = None,
) -> dict[str, Any]:
    """Apply Object Lock configuration to an S3 bucket.

    The bucket must have been created with Object Lock enabled
    (``--object-lock-enabled-for-bucket`` at creation time). This
    function sets the default retention policy so all new objects
    are automatically locked.

    Args:
        s3_client: An aiobotocore S3 client (or boto3 client).
        bucket: Bucket name.
        config: Lock configuration. Defaults to COMPLIANCE mode,
                2190 days (6 years, HIPAA-safe).

    Returns:
        Summary dict with applied settings.
    """
    if config is None:
        config = S3ObjectLockConfig()

    lock_config = {
        "ObjectLockEnabled": "Enabled",
        "Rule": {
            "DefaultRetention": {
                "Mode": config.mode,
                "Days": config.retention_days,
            }
        },
    }

    put = getattr(s3_client, "put_object_lock_configuration", None)
    if put is None:
        logger.warning(
            "S3 client does not support put_object_lock_configuration "
            "(MinIO/LocalStack may not support Object Lock)"
        )
        return {"status": "skipped", "reason": "client does not support Object Lock"}

    await put(Bucket=bucket, ObjectLockConfiguration=lock_config)

    logger.info(
        "S3 Object Lock configured: bucket=%s mode=%s retention=%dd",
        bucket,
        config.mode,
        config.retention_days,
    )

    return {
        "status": "applied",
        "bucket": bucket,
        "mode": config.mode,
        "retention_days": config.retention_days,
    }
