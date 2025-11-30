"""
Tasker TUI - Terminal User Interface for workflow monitoring.

Architecture:
- providers/: Data access layer (protocols + implementations)
- views/: Textual screen/widget components
- app.py: Main application entry point

Extensibility points:
1. New views: Add to views/, register in app.py
2. New data sources: Implement provider protocols
3. New widgets: Create composable widgets in views/widgets/
"""
