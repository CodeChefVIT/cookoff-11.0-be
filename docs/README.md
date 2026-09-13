# Cookoff 11.0 API Documentation

This directory contains system architecture specifications, OpenAPI 3.0 schemas, HLD/LLD documents, and API documentation for the Cookoff 11.0 backend project.

---

## 🚀 Deploying Documentation Statically (No Backend Required)

You can share these API docs with anyone without running Go, Docker, PostgreSQL, or Redis.

### Option 1: Vercel Deployment (Recommended)
1. Push your changes to GitHub.
2. Go to [Vercel Dashboard](https://vercel.com/new) -> **Add New Project** -> Import this repository.
3. In the project settings, set **Root Directory** to `docs`.
4. Click **Deploy**.
5. Your interactive Swagger UI will be live at `https://<your-project>.vercel.app`!

### Option 2: Render Deployment
1. Go to [Render Dashboard](https://dashboard.render.com) -> **New** -> **Static Site**.
2. Connect your GitHub repository.
3. Set **Root Directory** to `docs`.
4. Leave **Build Command** blank, set **Publish Directory** to `.`.
5. Click **Create Static Site**.

---

## 🖥️ Local Interactive Docs

### Option A: Open `index.html` via static web server
```bash
cd docs
python -m http.server 8000
```
Open `http://localhost:8000` in your browser.

### Option B: Via Go Server
Run the Go backend server (`make dev` or `go run cmd/api/main.go`) and visit:
```
http://localhost:8080/docs
```

---

## 📋 Documented API Categories
- **System & Health**: `/health`, `/docs`, `/judge0callback`
- **Authentication**: `/auth/google`, `/auth/google/callback`, `/refreshToken`, `/logout`
- **Dashboard & Attempts**: `/dashboard`, `/attempts/{id}`
- **Questions & Testcases**: `/question/round`, `/question/{id}`, `/question/{id}/blocks`, `/question/{id}/testcases/public`
- **Submissions & Execution**: `/submit`, `/submit/visual`, `/result/{submission_id}`, `/runcode`, `/runcustom`
- **Leaderboard**: `/leaderboard`, `/admin/leaderboard`
- **Admin Management**: User moderation, question CRUD & bounties, testcase CRUD, timer control, event analytics.
