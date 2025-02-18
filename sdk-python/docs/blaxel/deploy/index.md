Module beamlit.deploy
=====================
This module provides functions and classes to handle deployment processes within Beamlit.
It includes utilities to generate deployment configurations and manage deployment-related operations.

Sub-modules
-----------
* beamlit.deploy.deploy
* beamlit.deploy.format
* beamlit.deploy.parser

Functions
---------

`generate_beamlit_deployment(directory: str, name: str)`
:   Generates all necessary deployment files for Beamlit agents and functions.
    
    Args:
        directory (str): Target directory for generated files.
        name (str): Name identifier for the deployment.
    
    Creates:
        - Agent and function YAML configurations.
        - Dockerfiles for each deployment.
        - Directory structure for agents and functions.