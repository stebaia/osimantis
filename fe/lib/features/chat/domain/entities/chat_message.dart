import 'package:equatable/equatable.dart';

/// Chi ha prodotto il messaggio in chat.
enum ChatRole { user, assistant }

/// Un messaggio nella conversazione con l'agente. Entità di dominio: nessuna
/// dipendenza da Dio/JSON.
class ChatMessage extends Equatable {
  const ChatMessage({
    required this.role,
    required this.text,
    this.pending = false,
  });

  final ChatRole role;
  final String text;

  /// true mentre attendiamo la risposta dell'agente (bolla "sta scrivendo").
  final bool pending;

  ChatMessage copyWith({String? text, bool? pending}) => ChatMessage(
    role: role,
    text: text ?? this.text,
    pending: pending ?? this.pending,
  );

  @override
  List<Object?> get props => [role, text, pending];
}
