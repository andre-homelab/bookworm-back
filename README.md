# bookworm-back
<<<<<<< Updated upstream
API rest em GO
=======

Backend REST em Go para gerenciamento de livros pessoais.

## Requisitos
- Go 1.24+
- Docker + Docker Compose

## Execução
```bash
cp .env.example .env
make compose
make api
```

## Swagger
- UI: http://localhost:8080/swagger/index.html
- JSON: http://localhost:8080/swagger/doc.json

## Endpoint implementado
- `POST /api/v1/books`
- `GET /api/v1/books/search/isbn/{isbn}`
- `GET /api/v1/books/search?q=clean+code`
>>>>>>> Stashed changes
