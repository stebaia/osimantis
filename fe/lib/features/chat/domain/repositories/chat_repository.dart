import '../../../../core/error/result.dart';
import '../entities/chat_message.dart';
import '../entities/chat_reply.dart';

/// Contratto del repository di chat (domain). L'implementazione vive nel layer
/// data; il bloc dipende solo da questa astrazione.
abstract class ChatRepository {
  /// Invia il testo dell'utente all'agente e restituisce la sua risposta
  /// (testo + persone toccate). [history] è lo storico della conversazione
  /// (memoria a breve termine).
  Future<Result<ChatReply>> sendMessage(
    String text, {
    List<ChatMessage> history,
  });
}
