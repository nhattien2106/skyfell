"use client";
import { useEffect, useState } from "react";

interface PageInfo {
  id: number;
  url: string;
  title: string;
  meta: string;
  htmlVersion: string;
  headings: Record<string, number>;
  internalLinks: number;
  externalLinks: number;
  brokenLinks: number;
  loginForm: boolean;
}

export default function Dashboard() {
  const [pages, setPages] = useState<PageInfo[]>([]);
  const [url, setUrl] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  // Fetch all pages
  useEffect(() => {
    fetch("http://localhost:8080/api/pages")
      .then((res) => res.json())
      .then(setPages)
      .catch(() => setError("Failed to fetch data"));
  }, []);

  // Submit new URL
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError("");
    try {
      const res = await fetch("http://localhost:8080/api/crawl", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url }),
      });
      if (!res.ok) throw new Error("Failed to crawl URL");
      // Refresh table
      const newPages = await fetch("http://localhost:8080/api/pages").then(
        (r) => r.json()
      );
      setPages(newPages);
      setUrl("");
    } catch (err: any) {
      setError(err.message || "Unknown error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="max-w-4xl mx-auto p-4">
      <h1 className="text-2xl font-bold mb-4">Skyfell Dashboard</h1>
      <form onSubmit={handleSubmit} className="flex gap-2 mb-6">
        <input
          type="url"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder="Enter website URL"
          required
          className="border rounded px-2 py-1 flex-1"
        />
        <button
          type="submit"
          disabled={loading}
          className="bg-blue-600 text-white px-4 py-1 rounded"
        >
          {loading ? "Crawling..." : "Crawl"}
        </button>
      </form>
      {error && <p className="text-red-600 mb-2">{error}</p>}
      <div className="overflow-x-auto">
        <table className="min-w-full border">
          <thead>
            <tr className="bg-gray-100">
              <th className="p-2 border">Title</th>
              <th className="p-2 border">HTML Version</th>
              <th className="p-2 border">#Internal Links</th>
              <th className="p-2 border">#External Links</th>
              <th className="p-2 border">#Broken Links</th>
              <th className="p-2 border">Login Form</th>
              <th className="p-2 border">URL</th>
            </tr>
          </thead>
          <tbody>
            {pages.map((p) => (
              <tr key={p.id} className="hover:bg-gray-50">
                <td className="p-2 border">{p.title}</td>
                <td className="p-2 border">{p.htmlVersion}</td>
                <td className="p-2 border text-center">{p.internalLinks}</td>
                <td className="p-2 border text-center">{p.externalLinks}</td>
                <td className="p-2 border text-center">{p.brokenLinks}</td>
                <td className="p-2 border text-center">
                  {p.loginForm ? "Yes" : "No"}
                </td>
                <td className="p-2 border text-xs break-all">
                  <a
                    href={p.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-blue-600 underline"
                  >
                    {p.url}
                  </a>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
