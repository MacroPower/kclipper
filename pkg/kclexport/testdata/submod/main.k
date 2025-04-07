import foo

schema Config:
    """Configuration schema for testing the export functionality."""
    
    # Basic fields with different types
    name: str
    version: str
    enabled: bool = True
    priority: int = 0
    
    # Complex fields
    metadata?: {str: any}
    
    # Nested structure
    containers: [Container]

schema Container:
    """Container specification for the Config schema."""
    
    # Required fields
    name: str
    image: str
    
    # Optional fields
    command?: [str]
    args?: [str]
    env?: [Env]
    ports?: [int]

schema Env:
    name: str
    value: str

schema Random:
    """Schema for testing the export's ignore functionality."""
    
    bar: foo.Bar
