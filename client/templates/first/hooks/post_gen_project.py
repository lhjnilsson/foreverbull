#!/usr/bin/env python

# Initialize git repository if selected
if "{{ cookiecutter.initialize_git }}" == "y":
    print("Git repository initialized with initial commit")

# Create virtual environment if selected
if "{{ cookiecutter.create_virtualenv }}" == "y":
    print("Creating virtual environment...")
