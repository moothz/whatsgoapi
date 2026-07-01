/**
 * query-whatsgo.js
 * Ferramenta de debug para chamar a whatsmeow GO API diretamente.
 * Espelha todos os métodos this.apiClient.* de WhatsAppBotGo.js.
 *
 * Uso:
 *   node query-whatsgo.js <bot> <função> [args...]
 *
 * <bot>  Nome do bot definido em bots.json (campo "nome"). Obrigatório.
 *
 * Exemplos:
 *   node query-whatsgo.js mybot instanceStatus
 *   node query-whatsgo.js mybot groupInfo 120363000000000001@g.us
 *   node query-whatsgo.js mybot userInfo 5599999999999@s.whatsapp.net
 *   node query-whatsgo.js mybot blockList
 *   node query-whatsgo.js mybot listGroups
 *   node query-whatsgo.js mybot allInstances
 *   node query-whatsgo.js mybot sendText 5599999999999@s.whatsapp.net "Oi tudo bem?"
 *   node query-whatsgo.js mybot sendMedia 5599999999999@s.whatsapp.net https://example.com/img.jpg image
 *   node query-whatsgo.js mybot sendReaction 120363000@g.us MSG_ID_AQUI 👍
 *   node query-whatsgo.js mybot deleteMessage 120363000@g.us MSG_ID_AQUI true
 *   node query-whatsgo.js mybot groupLeave 120363000000000001@g.us
 *   node query-whatsgo.js mybot groupJoin INVITE_CODE
 *   node query-whatsgo.js mybot groupInviteInfo https://chat.whatsapp.com/XXXXXXXX
 *   node query-whatsgo.js mybot commonGroups 5599999999999@s.whatsapp.net
 *   node query-whatsgo.js mybot downloadMedia '{"imageMessage":{"url":"...","mimetype":"image/jpeg"}}'
 *   node query-whatsgo.js mybot profileStatus "Meu novo status"
 *   node query-whatsgo.js mybot updatePhoto https://example.com/foto.jpg
 *   node query-whatsgo.js mybot groupPhoto 120363000000000001@g.us https://example.com/foto.jpg
 *   node query-whatsgo.js mybot groupName 120363000000000001@g.us "Novo Nome do Grupo"
 *   node query-whatsgo.js mybot blockUser 5599999999999@s.whatsapp.net
 *   node query-whatsgo.js mybot unblockUser 5599999999999@s.whatsapp.net
 *   node query-whatsgo.js mybot instanceCreate 55596424307 3322
 *   node query-whatsgo.js mybot instanceCreate 55596424307 3322 pair
 *   node query-whatsgo.js mybot instanceConnect
 *   node query-whatsgo.js mybot instanceConnect http://host.docker.internal:3322 55596424307 pair
 *   node query-whatsgo.js mybot instancePair 55596424307
 *   node query-whatsgo.js mybot instanceQR
 *   node query-whatsgo.js mybot instanceLogout
 *   node query-whatsgo.js mybot instanceDelete
 *   node query-whatsgo.js mybot getCurrentGroups
 */

// Silencia os logs do dotenv (como "[dotenv@17.2.3] injecting env") se --json estiver ativo
const _log = console.log;
if (process.argv.includes("--json")) console.log = () => {};
require("dotenv").config();
if (process.argv.includes("--json")) console.log = _log;
const axios = require("axios");
const qrcodeTerminal = require("qrcode-terminal");
const { v4: uuidv4 } = require("uuid");

// ─── Configuração ──────────────────────────────────────────────────────────────
const BASE_URL = (process.env.WHATS_GO_API_URL || "http://localhost:9800").replace(/\/$/, "");
const GLOBAL_KEY = process.env.GLOBAL_API_KEY || "admin";

// Verifica se a opção --json foi passada e a remove dos argumentos
const isJsonOutput = process.argv.includes("--json");
const rawArgs = process.argv.filter((arg) => arg !== "--json");

// O nome do bot é o primeiro argumento posicional obrigatório
const BOT_NAME = rawArgs[2];

const { BOT_NOME } = (() => {
	const botsPath = "bots.json";
	const fs = require("fs");

	if (!BOT_NAME) {
		return { BOT_NOME: null };
	}

	if (fs.existsSync(botsPath)) {
		try {
			const bots = JSON.parse(fs.readFileSync(botsPath, "utf8"));
			const bot = bots.find((b) => b.usewhatsgo && b.nome === BOT_NAME);
			if (bot) {
				if (!bot.enabled) {
					console.warn(
						`[AVISO] Bot "${BOT_NAME}" existe mas não está habilitado (enabled: false). Continuando mesmo assim.`
					);
				}
				return { BOT_NOME: bot.nome };
			}
		} catch (err) {
			console.warn(`[AVISO] Falha ao ler ${botsPath}: ${err.message}. Usando nome fornecido.`);
		}
	}

	// Fallback se não houver bots.json ou o bot não estiver lá
	return { BOT_NOME: BOT_NAME };
})();

if (!BASE_URL || !GLOBAL_KEY) {
	console.error("[ERRO] Variáveis não encontradas. Verifique:");
	console.error("  WHATS_GO_API_URL =", BASE_URL || "(vazio)");
	console.error("  GLOBAL_API_KEY =", GLOBAL_KEY ? "(ok)" : "(vazio)");
	process.exit(1);
}

if (!isJsonOutput) {
	if (BOT_NOME) console.log(`[whatsgo Debug] Bot: ${BOT_NOME}`);
	console.log(`[whatsgo Debug] Servidor: ${BASE_URL}`);
	console.log(`[whatsgo Debug] GlobalKey: ${GLOBAL_KEY.substring(0, 8)}...`);
	console.log("─".repeat(60));
}

// ─── Cliente HTTP ───────────────────────────────────────────────────────────────
const client = axios.create({ baseURL: BASE_URL });

const HEADERS_GLOBAL = {
	apikey: GLOBAL_KEY,
	"Content-Type": "application/json",
	instance: BOT_NOME
};
const HEADERS_INSTANCE = {
	apikey: GLOBAL_KEY,
	"Content-Type": "application/json",
	instance: BOT_NOME
};

async function apiGet(endpoint, params = {}, useGlobalKey = false) {
	const headers = useGlobalKey ? HEADERS_GLOBAL : HEADERS_INSTANCE;
	const res = await client.get(endpoint, { params, headers });
	return res.data;
}

async function apiPost(endpoint, body = {}, useGlobalKey = false) {
	const headers = useGlobalKey ? HEADERS_GLOBAL : HEADERS_INSTANCE;
	const res = await client.post(endpoint, body, { headers });
	return res.data;
}

async function apiPut(endpoint, body = {}, useGlobalKey = false) {
	const headers = useGlobalKey ? HEADERS_GLOBAL : HEADERS_INSTANCE;
	const res = await client.put(endpoint, body, { headers });
	return res.data;
}

async function apiDelete(endpoint, body = {}, useGlobalKey = false) {
	const headers = useGlobalKey ? HEADERS_GLOBAL : HEADERS_INSTANCE;
	const res = await client.delete(endpoint, { headers, data: body });
	return res.data;
}

async function apiDeletePath(endpoint, useGlobalKey = false) {
	const headers = useGlobalKey ? HEADERS_GLOBAL : HEADERS_INSTANCE;
	const res = await client.delete(endpoint, { headers });
	return res.data;
}

// ─── Helpers ────────────────────────────────────────────────────────────────────
function dump(label, data) {
	if (isJsonOutput) {
		console.log(JSON.stringify(data, null, 2));
	} else {
		console.log(`\n[${label}]`);
		console.log(JSON.stringify(data, null, 2));
	}
}

function parseError(error) {
	const status = error.response?.status;
	const data = error.response?.data;
	const message = data?.message || error.message || "Erro desconhecido";
	console.error(`\n[ERRO HTTP ${status ?? "?"}] ${message}`);
	if (data) console.error("[Detalhes]", JSON.stringify(data, null, 2));
	console.error("[Stack]", error.stack ?? error);
}

// ─── Funções de API ─────────────────────────────────────────────────────────────

/** GET /instance/all — Lista todas as instâncias (usa GlobalKey) */
async function allInstances() {
	dump("allInstances", await apiGet("/instance/all", {}, true));
}

/** GET /instance/status — Status da instância */
async function instanceStatus() {
	dump("instanceStatus", await apiGet("/instance/status"));
}

/** POST /instance/connect — Conecta a instância */
async function instanceConnect(webhookUrl = null, phone = null, usePairing = false) {
	let url = webhookUrl;
	
	// Se não passou URL e não é explicitamente "none", usa o padrão de debug
	if (url === null) {
		url = `${process.env.WHATSGO_WEBHOOK_HOST || "http://host.docker.internal"}:${process.env.WHATSGO_WEBHOOK_PORT || "3000"}/webhook/whatsgo/debug`;
	} else if (url === "none" || url === "0") {
		url = ""; // Desabilita webhook
	}
	
	if (!isJsonOutput) {
		if (url) console.log(`[Connect] Iniciando conexão para "${BOT_NOME}" com webhook: ${url}`);
		else console.log(`[Connect] Iniciando conexão para "${BOT_NOME}" SEM webhook.`);
	}
	
	const connectRes = await apiPost("/instance/connect", {
		webhookUrl: url,
		phone: phone,
		subscribe: [
			"MESSAGE",
			"SEND_MESSAGE",
			"READ_RECEIPT",
			"PRESENCE",
			"CHAT_PRESENCE",
			"CALL",
			"CONNECTION",
			"LABEL",
			"CONTACT",
			"GROUP",
			"NEWSLETTER",
			"QRCODE"
		],
		websocketEnable: ""
	});

	if (isJsonOutput) {
		dump("instanceConnect", connectRes);
		return;
	}

	console.log(`[Connect] Resposta:`, connectRes.message);

	// Se for pairing code, solicita agora com retentativa
	if (usePairing && phone) {
		if (!isJsonOutput) console.log(`[Connect] Aguardando 10s para inicialização da instância...`);
		await new Promise((r) => setTimeout(r, 10000));

		let pCode = null;
		for (let i = 0; i < 5; i++) {
			if (!isJsonOutput) console.log(`[Connect] Tentativa ${i + 1}/5: Solicitando Pairing Code para ${phone}...`);
			pCode = await instancePair(phone);
			if (pCode && pCode !== "ERRO") break;
			if (!isJsonOutput) console.log(`[Connect] Ainda não pronto. Aguardando mais 5s...`);
			await new Promise((r) => setTimeout(r, 5000));
		}
	}

	// Inicia Polling
	console.log("[Connect] Aguardando " + (usePairing ? "Conexão" : "QR Code") + "...");
	let lastQR = "";
	let attempts = 0;
	const maxAttempts = 40; // ~2 minutos

	while (attempts < maxAttempts) {
		attempts++;
		try {
			// 1. Verifica Status
			const status = await apiGet("/instance/status");
			if (status.data?.LoggedIn) {
				console.log(`\n✅ Instância "${BOT_NOME}" CONECTADA com sucesso!`);
				console.log(`JID: ${status.data.myJid || "vazio"}`);
				return;
			}

			// 2. Verifica QR (apenas se não for pairing)
			if (!usePairing) {
				const qrRes = await apiGet("/instance/qr").catch(() => null);
				if (qrRes && qrRes.data?.Code && qrRes.data.Code !== lastQR) {
					lastQR = qrRes.data.Code;
					console.log("\n" + "─".repeat(40));
					console.log("NOVO QR CODE RECEBIDO:");
					qrcodeTerminal.generate(lastQR, { small: true });
					console.log("Escaneie o código acima no seu WhatsApp.");
					console.log("─".repeat(40) + "\n");
				}
			}
		} catch (e) {
			// Silencia erros de polling
		}

		process.stdout.write(".");
		await new Promise((r) => setTimeout(r, 3000));
	}

	console.log("\n[Aviso] Tempo limite de espera atingido. Verifique o status manualmente.");
}

/** POST /instance/pair — Solicita pairing code */
async function instancePair(phone) {
	if (!phone) {
		console.error("[ERRO] Telefone obrigatório para pairing.");
		return null;
	}
	try {
		const res = await apiPost("/instance/pair", { phone });
		const code = res.data?.PairingCode;
		
		if (!isJsonOutput) {
			if (code) {
				console.log(`\nPairing Code para ${phone}:`);
				console.log("─".repeat(20));
				console.log(`   ${code}`);
				console.log("─".repeat(20) + "\n");
			}
		} else {
			dump("instancePair", res);
		}
		return code || "ERRO";
	} catch (e) {
		if (!isJsonOutput) console.warn(`[Aviso] Falha ao solicitar pairing code: ${e.message}`);
		return "ERRO";
	}
}

/** GET /instance/qr — Retorna QR Code atual */
async function instanceQR() {
	const res = await apiGet("/instance/qr");
	if (!isJsonOutput && res.data?.Code) {
		qrcodeTerminal.generate(res.data.Code, { small: true });
	} else {
		dump("instanceQR", res);
	}
}

/** DELETE /instance/logout — Faz logout da instância */
async function instanceLogout() {
	dump("instanceLogout", await apiDelete("/instance/logout", {}, false));
}

/** DELETE /instance/delete/:id — Deleta instância (Admin) */
async function instanceDelete(id) {
	const targetName = id || BOT_NOME;
	if (!targetName) {
		console.error("[ERRO] Nome ou ID da instância não informado.");
		return;
	}

	if (!isJsonOutput) console.log(`[Delete] Buscando UUID para a instância "${targetName}"...`);

	try {
		const all = await apiGet("/instance/all", {}, true);
		const instance = all.data?.find((i) => i.name === targetName || i.id === targetName);

		if (!instance) {
			console.error(`[ERRO] Instância "${targetName}" não encontrada.`);
			return;
		}

		const uuid = instance.id;
		if (!isJsonOutput) console.log(`[Delete] UUID encontrado: ${uuid}. Deletando...`);
		
		dump("instanceDelete", await apiDeletePath(`/instance/delete/${uuid}`, true));
	} catch (e) {
		parseError(e);
	}
}

/** POST /instance/create — Cria nova instância (usa GlobalKey) */
async function instanceCreate(name, phone = null, port = null, usePairing = false) {
	if (!name) {
		console.error("[ERRO] Nome da instância é obrigatório.");
		return;
	}

	if (!isJsonOutput) console.log(`[Create] Criando instância "${name}"...`);

	const token = uuidv4();
	const instanceId = uuidv4();
	const createRes = await apiPost(
		"/instance/create",
		{
			name,
			instanceId,
			token
		},
		true
	);

	if (!isJsonOutput) {
		console.log("Instância criada com sucesso.");
		dump("instanceCreate", createRes);
	}

	// Se passou telefone, já tenta conectar
	if (phone) {
		let webhookUrl = "none"; // Padrão: sem webhook se não informar porta
		if (port && port !== "0" && port !== "none") {
			webhookUrl = `http://host.docker.internal:${port}`;
		}
		await instanceConnect(webhookUrl, phone, usePairing);
	}
}

/** PUT /instance/:id/advanced-settings — Configurações avançadas da instância */
async function instanceAdvSettings(instanceId, opts = {}) {
	const body = {
		alwaysOnline: opts.alwaysOnline ?? true,
		rejectCall: opts.rejectCall ?? true,
		readMessages: opts.readMessages ?? false,
		ignoreGroups: opts.ignoreGroups ?? false,
		ignoreStatus: opts.ignoreStatus ?? true
	};
	dump("instanceAdvSettings", await apiPut(`/instance/${instanceId}/advanced-settings`, body));
}

/** POST /instance/passkey/respond — Responde com a assinatura do Passkey */
async function instancePasskeyRespond(responseJsonStr) {
	const response = typeof responseJsonStr === "string" ? JSON.parse(responseJsonStr) : responseJsonStr;
	dump("instancePasskeyRespond", await apiPost("/instance/passkey/respond", response));
}

/** POST /instance/passkey/confirm — Confirma o passkey se necessário */
async function instancePasskeyConfirm() {
	dump("instancePasskeyConfirm", await apiPost("/instance/passkey/confirm", {}));
}

/** GET /group/list — Lista grupos */
async function listGroups() {
	dump("listGroups", await apiGet("/group/list"));
}

/** GET /group/myall — Grupos em que o bot está */
async function getCurrentGroups() {
	dump("getCurrentGroups", await apiGet("/group/myall"));
}

/** POST /group/info — Info de um grupo */
async function groupInfo(groupJid) {
	dump("groupInfo", await apiPost("/group/info", { groupJid }));
}

/** POST /group/leave — Sai de um grupo */
async function groupLeave(groupJid) {
	dump("groupLeave", await apiPost("/group/leave", { groupJid }));
}

/** POST /group/join — Entra em um grupo por código de convite */
async function groupJoin(code) {
	dump("groupJoin", await apiPost("/group/join", { code }));
}

/** POST /group/invite-info — Info sobre um link de convite */
async function groupInviteInfo(code) {
	const inviteLink = code.includes("chat.whatsapp.com")
		? code
		: `https://chat.whatsapp.com/${code}`;
	dump("groupInviteInfo", await apiPost("/group/invite-info", { code: inviteLink }));
}

/** POST /group/name — Renomeia o grupo */
async function groupName(groupJid, name) {
	dump("groupName", await apiPost("/group/name", { groupJid, name }));
}

/** POST /group/photo — Atualiza foto do grupo */
async function groupPhoto(groupJid, imageUrlOrBase64) {
	dump("groupPhoto", await apiPost("/group/photo", { groupJid, image: imageUrlOrBase64 }));
}

/** POST /user/info — Info de um usuário */
async function userInfo(number) {
	// number deve ter @s.whatsapp.net
	const jid = number.includes("@") ? number : `${number}@s.whatsapp.net`;
	dump("userInfo", await apiPost("/user/info", { number: [jid] }));
}

/** GET /user/blocklist — Lista de contatos bloqueados */
async function blockList() {
	dump("blockList", await apiGet("/user/blocklist"));
}

/** POST /user/block — Bloqueia um usuário */
async function blockUser(number) {
	const jid = number.includes("@") ? number : `${number}@s.whatsapp.net`;
	dump("blockUser", await apiPost("/user/block", { number: jid }));
}

/** POST /user/unblock — Desbloqueia um usuário */
async function unblockUser(number) {
	const jid = number.includes("@") ? number : `${number}@s.whatsapp.net`;
	dump("unblockUser", await apiPost("/user/unblock", { number: jid }));
}

/** POST /user/profileStatus — Atualiza o status do perfil */
async function profileStatus(status) {
	dump("profileStatus", await apiPost("/user/profileStatus", { status }));
}

/** POST /user/photo — Atualiza a foto de perfil */
async function updatePhoto(imageUrlOrBase64) {
	dump("updatePhoto", await apiPost("/user/photo", { image: imageUrlOrBase64 }));
}

/** POST /chat/commonGroups — Grupos em comum com um usuário */
async function commonGroups(jid) {
	const jidCom = jid.includes("@") ? jid : `${jid}@s.whatsapp.net`;
	dump("commonGroups", await apiPost("/chat/commonGroups", { jid: jidCom }));
}

/** POST /send/text — Envia mensagem de texto */
async function sendText(number, text, quotedMessageId = null) {
	const body = { number, text, delay: 0 };
	if (quotedMessageId) body.quoted = { messageId: quotedMessageId };
	dump("sendText", await apiPost("/send/text", body));
}

/** POST /send/media — Envia mídia por URL */
async function sendMedia(number, url, type = "image", caption = null) {
	const body = { number, url, type, delay: 0 };
	if (caption) body.caption = caption;
	dump("sendMedia", await apiPost("/send/media", body));
}

/** POST /send/sticker — Envia sticker por URL */
async function sendSticker(number, stickerUrl) {
	dump("sendSticker", await apiPost("/send/sticker", { number, sticker: stickerUrl, delay: 0 }));
}

/** POST /send/location — Envia localização */
async function sendLocation(number, latitude, longitude, name = "", address = "") {
	dump(
		"sendLocation",
		await apiPost("/send/location", { number, latitude, longitude, name, address, delay: 0 })
	);
}

/** POST /send/contact — Envia contato */
async function sendContact(number, fullName, phone) {
	dump(
		"sendContact",
		await apiPost("/send/contact", { number, vcard: { fullName, phone }, delay: 0 })
	);
}

/** POST /send/poll — Envia enquete */
async function sendPoll(number, question, options, maxAnswer = 1) {
	dump("sendPoll", await apiPost("/send/poll", { number, question, options, maxAnswer, delay: 0 }));
}

/** POST /message/react — Envia reação a uma mensagem */
async function sendReaction(number, messageId, reaction) {
	dump(
		"sendReaction",
		await apiPost("/message/react", { number, reaction, id: messageId, fromMe: false })
	);
}

/** POST /message/delete — Deleta uma mensagem */
async function deleteMessage(chat, messageId, fromMe = true, participant = null) {
	const body = { chat, messageId, fromMe };
	if (participant) body.participant = participant;
	dump("deleteMessage", await apiPost("/message/delete", body));
}

/** POST /message/downloadmedia — Faz download de mídia de uma mensagem */
async function downloadMedia(messageJson) {
	// messageJson: objeto message ou string JSON representando o campo Message da mensagem
	const message = typeof messageJson === "string" ? JSON.parse(messageJson) : messageJson;
	dump("downloadMedia", await apiPost("/message/downloadmedia", { message }));
}

// ─── Dispatch ───────────────────────────────────────────────────────────────────
const FUNCTIONS = {
	allInstances: () => allInstances(),
	listBots: () => allInstances(),
	instanceStatus: () => instanceStatus(),
	instanceConnect: ([webhookUrl, phone, pairing]) =>
		instanceConnect(webhookUrl, phone, pairing === "pair" || pairing === "true"),
	instancePair: ([phone]) => instancePair(phone),
	instanceQR: () => instanceQR(),
	instanceLogout: () => instanceLogout(),
	instanceDelete: ([id]) => instanceDelete(id),
	instanceCreate: ([phone, port, pairing]) =>
		instanceCreate(BOT_NOME, phone, port, pairing === "pair" || pairing === "true"),
	instanceAdvSettings: ([instanceId]) => instanceAdvSettings(instanceId),
	instancePasskeyRespond: ([responseJson]) => instancePasskeyRespond(responseJson),
	instancePasskeyConfirm: () => instancePasskeyConfirm(),

	listGroups: () => listGroups(),
	getCurrentGroups: () => getCurrentGroups(),
	groupInfo: ([groupJid]) => groupInfo(groupJid),
	groupLeave: ([groupJid]) => groupLeave(groupJid),
	groupJoin: ([code]) => groupJoin(code),
	groupInviteInfo: ([code]) => groupInviteInfo(code),
	groupName: ([groupJid, name]) => groupName(groupJid, name),
	groupPhoto: ([groupJid, imageUrl]) => groupPhoto(groupJid, imageUrl),

	userInfo: ([number]) => userInfo(number),
	blockList: () => blockList(),
	blockUser: ([number]) => blockUser(number),
	unblockUser: ([number]) => unblockUser(number),
	profileStatus: ([status]) => profileStatus(status),
	updatePhoto: ([imageUrl]) => updatePhoto(imageUrl),
	commonGroups: ([jid]) => commonGroups(jid),

	sendText: ([number, text, quotedId]) => sendText(number, text, quotedId),
	sendMedia: ([number, url, type, caption]) => sendMedia(number, url, type, caption),
	sendSticker: ([number, url]) => sendSticker(number, url),
	sendLocation: ([number, lat, lon, name, addr]) =>
		sendLocation(number, parseFloat(lat), parseFloat(lon), name, addr),
	sendContact: ([number, fullName, phone]) => sendContact(number, fullName, phone),
	sendPoll: ([number, question, ...opts]) => sendPoll(number, question, opts),
	sendReaction: ([number, messageId, reaction]) => sendReaction(number, messageId, reaction),
	deleteMessage: ([chat, messageId, fromMe, participant]) =>
		deleteMessage(chat, messageId, fromMe !== "false", participant),
	downloadMedia: ([messageJson]) => downloadMedia(messageJson)
};

async function main() {
	// rawArgs[2] = bot name, rawArgs[3] = função, rawArgs[4+] = args
	const botNameArg = rawArgs[2];
	const fnName = rawArgs[3];
	const args = rawArgs.slice(4);

	// Se nenhum argumento for passado, lista todos os bots (allInstances)
	if (!botNameArg) {
		if (!isJsonOutput) console.log("[whatsgo Debug] Nenhum bot informado. Listando todas as instâncias...");
		try {
			await allInstances();
		} catch (error) {
			parseError(error);
			process.exit(1);
		}
		return;
	}

	if (!fnName) {
		console.log("Funções disponíveis:\n");
		Object.keys(FUNCTIONS).forEach((f) => console.log(`  node query-whatsgo.js ${BOT_NOME} ${f}`));
		console.log("\nVeja exemplos no topo do arquivo.");
		process.exit(0);
	}

	const fn = FUNCTIONS[fnName];
	if (!fn) {
		console.error(`[ERRO] Função desconhecida: "${fnName}"`);
		console.error("Funções disponíveis:", Object.keys(FUNCTIONS).join(", "));
		process.exit(1);
	}

	try {
		await fn(args);
	} catch (error) {
		parseError(error);
		process.exit(1);
	}
}

main();
