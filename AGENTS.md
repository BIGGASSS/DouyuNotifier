# Agent Guidelines for DouyuNotifier

## Build/Run Commands

```bash
# Create a virtual environment
uv venv

# Activate the virtual environment
source .venv/bin/activate

# Install dependencies
uv pip install -r requirements.txt

# Run the application
uv run python main.py
```

## Test Commands

This project currently has no tests. When adding tests:

```bash
# Install pytest
uv pip install pytest pytest-cov

# Run all tests
uv run pytest

# Run a single test file
uv run pytest test_fetcher.py

# Run a specific test
uv run pytest test_fetcher.py::test_parse_response

# Run with coverage
uv run pytest --cov=. --cov-report=term-missing
```

## Lint/Format Commands

Recommended tools for this project:

```bash
# Install linting/formatting tools
uv pip install ruff mypy

# Format code
uv run ruff format .

# Check linting
uv run ruff check .

# Fix auto-fixable lint issues
uv run ruff check . --fix

# Type checking
uv run mypy .
```

## Code Style Guidelines

### Imports
- Order: stdlib → third-party → local modules
- Use absolute imports for local modules (e.g., `from config import POLL_INTERVAL`)
- Group imports with a blank line between each group
- Use `from typing import` for type hints

Example:
```python
import time
import sys
from typing import Set, Dict, List, Optional

import requests

from config import POLL_INTERVAL
from models import Room
```

### Naming Conventions
- **Variables/Functions**: `snake_case` (e.g., `get_douyu_cookies`, `previous_live`)
- **Classes**: `PascalCase` (e.g., `Room`, `NotLoginError`)
- **Constants**: `UPPER_CASE` (e.g., `POLL_INTERVAL`, `API_URL`)
- **Private functions**: `_leading_underscore` (e.g., `_extract_from_browser`)
- **Module-level constants**: `UPPER_CASE` in config.py

### Type Hints
- Use type hints for all function parameters and return values
- Use `Optional[T]` for values that may be None
- Use `Dict[str, T]` for dictionaries with known key types
- Use `List[T]` for lists, `Set[T]` for sets

Example:
```python
def fetch_douyu_live_status(cookies: Dict[str, str]) -> List[Room]:
    ...

def print_status(rooms: List[Room], previous_live: Optional[Set[str]] = None) -> Set[str]:
    ...
```

### Formatting
- 4 spaces for indentation
- Maximum line length: 88-100 characters
- Use f-strings for string formatting (e.g., `f"Hello {name}"`)
- Single quotes for strings by default
- Trailing commas in multi-line collections

### Documentation
- Module docstrings explaining purpose
- Function docstrings with Args, Returns, Raises sections when applicable
- Inline comments for complex logic

Example:
```python
def parse_response(data: Dict[str, Any]) -> List[Room]:
    """
    Parse Douyu API response into Room objects.
    
    Args:
        data: Raw API response dictionary
        
    Returns:
        List of Room objects
    """
```

### Error Handling
- Define custom exceptions in `models.py`
- Use specific exceptions: `NotLoginError`, `DouyuAPIError`
- Handle expected errors gracefully with retry logic
- Use try/except blocks for external I/O (network, file system)
- Log warnings for non-fatal errors

Example:
```python
try:
    response = requests.get(url, timeout=30)
    response.raise_for_status()
except requests.exceptions.RequestException as e:
    raise DouyuAPIError(f"Request failed: {e}")
```

### Data Classes
- Use `@dataclass` for data containers
- Define `__str__` for readable string representation
- Include type hints for all fields

Example:
```python
@dataclass
class Room:
    room_id: str
    room_name: str
    streamer_name: str
    is_live: bool
    # ...
```

### Configuration
- Store constants in `config.py`
- Include comments explaining units (e.g., `# 3 minutes in seconds`)
- Keep API URLs, intervals, and platform-specific values configurable

## Project Structure

```
DouyuNotifier/
├── main.py           # Entry point with polling loop
├── fetcher.py        # API client and response parser
├── models.py         # Data classes and exceptions
├── auth.py           # Cookie extraction and management
├── notifier.py       # Console output formatting
├── config.py         # Settings and constants
├── requirements.txt  # Dependencies
└── cookies.json      # Auto-generated cookie storage (gitignored)
```

## Dependencies

Core dependencies:
- `requests>=2.31.0` - HTTP library
- `browser-cookie3>=0.19.0` - Browser cookie extraction (optional)

Python version: 3.7+

## Notes

- This project fetches data from Douyu's private API using browser cookies
- No existing test suite - add pytest tests for new features
- No existing CI/CD configuration
- Code uses emoji in console output (allowed for this project)
