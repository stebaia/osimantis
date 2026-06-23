import 'package:dio/dio.dart';

import '../../domain/entities/person_card.dart';

/// Datasource remoto della scheda persona: parla con GET /people/{id} del
/// backend Go, che ritorna `WikiPageData`. Propaga DioException: la mappatura a
/// Failure la fa il repository.
class PersonRemoteDataSource {
  PersonRemoteDataSource(this._dio);
  final Dio _dio;

  Future<PersonCard> getPerson(int id) async {
    final res = await _dio.get<Map<String, dynamic>>('/people/$id');
    final json = res.data;
    if (json == null) {
      throw DioException(
        requestOptions: res.requestOptions,
        response: res,
        type: DioExceptionType.badResponse,
        error: 'Scheda persona vuota',
      );
    }
    return _fromJson(json);
  }

  PersonCard _fromJson(Map<String, dynamic> json) {
    return PersonCard(
      id: (json['id'] as num).toInt(),
      name: (json['name'] as String?) ?? '',
      aliases: _stringList(json['aliases']),
      data: (json['data'] as Map?)?.cast<String, dynamic>() ?? const {},
      neighbors: _list(json['neighbors'], _relationFromJson),
      events: _list(json['events'], _eventFromJson),
    );
  }

  PersonRelation _relationFromJson(Map<String, dynamic> j) => PersonRelation(
    nodeId: (j['node_id'] as num?)?.toInt() ?? 0,
    name: (j['name'] as String?) ?? '',
    relation: (j['relation'] as String?) ?? '',
    nodeType: (j['node_type'] as String?) ?? 'person',
    note: (j['note'] as String?) ?? '',
  );

  PersonEvent _eventFromJson(Map<String, dynamic> j) => PersonEvent(
    id: (j['id'] as num?)?.toInt() ?? 0,
    rawText: (j['raw_text'] as String?) ?? '',
    summary: j['summary'] as String?,
    occurredAt: j['occurred_at'] as String?,
  );

  List<String> _stringList(dynamic v) =>
      (v as List?)?.map((e) => e.toString()).toList() ?? const [];

  List<T> _list<T>(dynamic v, T Function(Map<String, dynamic>) f) =>
      (v as List?)
          ?.map((e) => f((e as Map).cast<String, dynamic>()))
          .toList() ??
      const [];
}
