part of 'chat_bloc.dart';

/// Stato della home blob-first.
///
/// - [status] guida il blob (idle/ascolto/invio) e il composer.
/// - [transcript] è il testo dettato in tempo reale.
/// - [messages] è lo storico conversazionale (memoria a breve termine inviata al
///   backend): NON viene reso come chat, serve solo a mantenere il filo.
/// - [activePerson] è la scheda mostrata sotto al blob (l'ultima persona toccata).
/// - [personStatus] guida lo stato della scheda (vuota/in caricamento/caricata).
/// - [lastReply] è la risposta testuale breve dell'agente, mostrata quando il
///   turno non tocca nessuna persona (nessuna scheda da aprire).
class ChatState extends Equatable {
  const ChatState({
    this.messages = const [],
    this.status = ChatStatus.idle,
    this.transcript = '',
    this.activePerson,
    this.personStatus = PersonStatus.none,
    this.lastReply = '',
    this.errorMessage,
  });

  final List<ChatMessage> messages;
  final ChatStatus status;
  final String transcript;
  final PersonCard? activePerson;
  final PersonStatus personStatus;
  final String lastReply;
  final String? errorMessage;

  bool get isListening => status == ChatStatus.listening;
  bool get isSending => status == ChatStatus.sending;

  /// Il blob è "attivo" (più grande, animazione veloce) mentre ascolta o elabora.
  bool get isBusy => isListening || isSending;

  ChatState copyWith({
    List<ChatMessage>? messages,
    ChatStatus? status,
    String? transcript,
    PersonCard? activePerson,
    PersonStatus? personStatus,
    String? lastReply,
    String? errorMessage,
    bool clearError = false,
  }) {
    return ChatState(
      messages: messages ?? this.messages,
      status: status ?? this.status,
      transcript: transcript ?? this.transcript,
      activePerson: activePerson ?? this.activePerson,
      personStatus: personStatus ?? this.personStatus,
      lastReply: lastReply ?? this.lastReply,
      errorMessage: clearError ? null : (errorMessage ?? this.errorMessage),
    );
  }

  @override
  List<Object?> get props => [
    messages,
    status,
    transcript,
    activePerson,
    personStatus,
    lastReply,
    errorMessage,
  ];
}

enum ChatStatus { idle, listening, sending }

/// Stato della scheda persona mostrata sotto al blob.
enum PersonStatus { none, loading, loaded }
