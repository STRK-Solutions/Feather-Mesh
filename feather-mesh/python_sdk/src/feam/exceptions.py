"""Typed errors from the versioned `feam.peer.v1` subprocess protocol."""


class FeamError(RuntimeError):
    """Base error returned by the Feather Mesh resolver."""

    def __init__(self, message: str, *, kind: str = "runtime_error", returncode: int | None = None):
        super().__init__(message)
        self.kind = kind
        self.returncode = returncode


class ValidationError(FeamError):
    pass


class NotFoundError(FeamError):
    pass


class PolicyError(FeamError):
    pass


class PeerUnavailableError(PolicyError):
    pass


class WithdrawnVersionError(PolicyError):
    pass


class IntegrityError(PolicyError):
    pass


class PublicationConflictError(FeamError):
    pass


class ProtocolError(FeamError):
    pass


class ExecutableNotFoundError(FeamError):
    pass


_ERRORS = {
    "bad_metadata": ValidationError,
    "unsupported_format": ValidationError,
    "not_found": NotFoundError,
    "dataset_not_registered": NotFoundError,
    "policy_failure": PolicyError,
    "peer_unavailable": PeerUnavailableError,
    "withdrawn_version": WithdrawnVersionError,
    "integrity_failed": IntegrityError,
    "publication_conflict": PublicationConflictError,
    "project_config_error": ProtocolError,
    "malformed_manifest": ProtocolError,
}


def error_for(kind: str, message: str, returncode: int | None = None) -> FeamError:
    return _ERRORS.get(kind, FeamError)(message, kind=kind, returncode=returncode)
