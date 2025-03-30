from setuptools import setup, find_packages

setup(
    name="blender_fetcher",
    version="0.1.0",
    packages=find_packages(),
    install_requires=[
        "rich>=13.9.4",
    ],
    entry_points={
        "console_scripts": [
            "fetch-blender-build=blender_fetcher:main",
            "fbb=blender_fetcher:main",
        ],
    },
    author="David Shot",
    description="A utility for finding, downloading, and managing Blender builds",
    long_description=open("README.md").read(),
    long_description_content_type="text/markdown",
    python_requires=">=3.6",
)
