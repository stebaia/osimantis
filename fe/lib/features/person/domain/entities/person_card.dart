import 'package:equatable/equatable.dart';

/// Scheda completa di una persona: dati anagrafici liberi, legami ed eventi.
/// Rispecchia `WikiPageData` del backend (GET /people/{id}). Entità di dominio:
/// nessuna dipendenza da Dio/JSON.
class PersonCard extends Equatable {
  const PersonCard({
    required this.id,
    required this.name,
    this.aliases = const [],
    this.data = const {},
    this.neighbors = const [],
    this.events = const [],
  });

  final int id;
  final String name;
  final List<String> aliases;

  /// Campi liberi (lavoro, città, interessi, ...) come chiave→valore.
  final Map<String, dynamic> data;
  final List<PersonRelation> neighbors;
  final List<PersonEvent> events;

  @override
  List<Object?> get props => [id, name, aliases, data, neighbors, events];
}

/// Un legame della persona verso un altro nodo (persona o luogo).
class PersonRelation extends Equatable {
  const PersonRelation({
    required this.nodeId,
    required this.name,
    required this.relation,
    this.nodeType = 'person',
    this.note = '',
  });

  final int nodeId;
  final String name;

  /// Etichetta del legame (es. 'amico', 'frequenta', 'ex').
  final String relation;
  final String nodeType;
  final String note;

  @override
  List<Object?> get props => [nodeId, name, relation, nodeType, note];
}

/// Un evento recente che coinvolge la persona.
class PersonEvent extends Equatable {
  const PersonEvent({
    required this.id,
    required this.rawText,
    this.summary,
    this.occurredAt,
  });

  final int id;
  final String rawText;
  final String? summary;
  final String? occurredAt;

  @override
  List<Object?> get props => [id, rawText, summary, occurredAt];
}
