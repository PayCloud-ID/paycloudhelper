# S3MinIO Stub Change Log

Track proto and generated-stub level changes here.

## Entry Template
- Date:
- Provider proto commit:
- Helper commit:
- Affected RPCs:
- Affected message fields:
- Required consumer action:
- Backward compatibility notes:

## Entries
- Date: 2026-04-12
	- Provider proto commit: workspace local (pending release tag)
	- Helper commit: workspace local (phase 3-5 implementation)
	- Affected RPCs: Upload, Download, GeneratePresignedUrl, GenerateViewUrl, Health
	- Affected message fields: none (no wire-contract field changes in this step)
	- Required consumer action: switch imports to `sdk/services/s3minio/*` and stop using legacy `phs3*` paths
	- Backward compatibility notes: runtime behavior preserved; package paths consolidated
