/**
 * listen-whatsgo.js
 * Simple listener for whatsgoapi webhooks.
 *
 * Uso:
 *   node listen-whatsgo.js [porta]
 *
 * Porta padrão: 3000
 */

const express = require("express");
const bodyParser = require("body-parser");

const app = express();
const PORT = process.argv[2] || 3000;

app.use(bodyParser.json({ limit: "50mb" }));
app.use(bodyParser.urlencoded({ extended: true, limit: "50mb" }));

app.use((req, res) => {
	const timestamp = new Date().toISOString();
	if (req.method === "POST") {
		console.log(`\n[${timestamp}] Webhook Received: ${req.path}`);
		console.log("Headers:", JSON.stringify(req.headers, null, 2));
		console.log("Body:", JSON.stringify(req.body, null, 2));
		console.log("─".repeat(60));
		return res.status(200).send("OK");
	}

	console.log(`[${timestamp}] ${req.method} Request to ${req.path}`);
	res.status(200).send("Webhook listener is running. Use POST to send data.");
});

app.listen(PORT, "0.0.0.0", () => {
	console.log(`\n🚀 Webhook listener running on http://0.0.0.0:${PORT}`);
	console.log(`Configure seu bot para enviar webhooks para este endereço.`);
	console.log(`Pressione Ctrl+C para encerrar.`);
	console.log("─".repeat(60));
});
