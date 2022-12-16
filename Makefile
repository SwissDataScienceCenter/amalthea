.PHONY: style-fix style-check unit-tests

style-fix:
	poetry run isort .
	poetry run black .

style-check:
	poetry run flake8 .
	poetry run isort . --check --diff 
	poetry run black . --check --diff

unit-tests:
	poetry run pytest tests/unit
