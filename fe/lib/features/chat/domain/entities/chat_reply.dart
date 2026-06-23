import 'package:equatable/equatable.dart';

/// Risposta dell'agente a un messaggio: il testo e i nodi persona toccati nel
/// turno. Il frontend apre la scheda dell'ULTIMO toccato. [touched] è vuoto se il
/// messaggio non ha riguardato nessuna persona.
class ChatReply extends Equatable {
  const ChatReply({required this.text, this.touched = const []});

  final String text;
  final List<TouchedPerson> touched;

  /// L'ultima persona toccata, di cui aprire la scheda. null se nessuna.
  TouchedPerson? get lastTouched => touched.isEmpty ? null : touched.last;

  @override
  List<Object?> get props => [text, touched];
}

/// Riferimento minimo a una persona toccata in un turno (id + nome).
class TouchedPerson extends Equatable {
  const TouchedPerson({required this.id, required this.name});

  final int id;
  final String name;

  @override
  List<Object?> get props => [id, name];
}
