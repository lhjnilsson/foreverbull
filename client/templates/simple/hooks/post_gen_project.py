#!/usr/bin/env python

# Create virtual environment if selected
if "{{ cookiecutter.create_virtualenv }}":
    print("Creating virtual environment...")
