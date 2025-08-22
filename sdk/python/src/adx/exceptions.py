"""
Exception definitions for ADX SDK
"""


class ADXException(Exception):
    """Base exception for ADX SDK"""
    pass


class AuthenticationError(ADXException):
    """Raised when authentication fails"""
    pass


class RateLimitError(ADXException):
    """Raised when rate limit is exceeded"""
    pass


class ValidationError(ADXException):
    """Raised when validation fails"""
    pass


class NetworkError(ADXException):
    """Raised when network operation fails"""
    pass